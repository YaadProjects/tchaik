// Copyright 2015, David Howden
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
tchremote is a tool which uses the Tchaik REST API to act as a remote control. See --help for more details.
*/
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	HostEnv      = "TCH_ADDR"
	PlayerKeyEnv = "TCH_PLAYER_KEY"
)

var host string
var key string
var keys bool
var action string
var value string
var create string
var delete bool

func init() {
	flag.StringVar(&host, "addr", "", fmt.Sprintf("schema://host(:port) `address` of the REST API (or set %v)", HostEnv))
	flag.StringVar(&key, "key", "", fmt.Sprintf("`key` which identifies the player to send actions to (or set %v)", PlayerKeyEnv))
	flag.BoolVar(&keys, "keys", false, "list all the keys on the host")
	flag.StringVar(&action, "action", "", "`action` to send to the player (requires -key, some require -value)")
	flag.StringVar(&value, "value", "", "`value` to send to the player")
	flag.StringVar(&create, "create", "", "create a multi-player from a comma-separeted `list` for the given -key")
	flag.BoolVar(&delete, "delete", false, "delete the player for -key")
}

func main() {
	flag.Parse()

	if host == "" {
		host = os.Getenv(HostEnv)
	}
	if host == "" {
		fmt.Printf("must use -addr or set %v\n", HostEnv)
		os.Exit(1)
	}

	err := handleParams()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func handleParams() error {
	if keys {
		pks, err := getPlayerKeys()
		if err != nil {
			return err
		}

		b, err := json.MarshalIndent(pks, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshalling keys: %v\n", err)
		}
		fmt.Println(string(b))
		return nil
	}

	if key == "" {
		key = os.Getenv(PlayerKeyEnv)
	}
	if key == "" {
		return fmt.Errorf("must use -key or set %v\n", PlayerKeyEnv)
	}

	var err error
	switch {
	case create != "":
		err = handleCreate(key, create)
		if err != nil {
			err = fmt.Errorf("error creating player key: %v\n", err)
		}
	case action != "":
		err = handleAction(action, value)
		if err != nil {
			err = fmt.Errorf("error handling action: %v\n", err)
		}
	case delete:
		err = handleDelete(key)
		if err != nil {
			err = fmt.Errorf("error deleting key: %v\n", err)
		}
	default:
		var p *Player
		p, err = getPlayer(key)
		if err != nil {
			err = fmt.Errorf("error fetching key: %v\n", err)
			break
		}
		b, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			err = fmt.Errorf("error marshalling player: %v\n", err)
			break
		}
		fmt.Println(string(b))
	}
	return err
}

func handleCreate(key, create string) error {
	keys := strings.Split(create, ",")
	data := struct {
		Key        string   `json:"key"`
		PlayerKeys []string `json:"playerKeys"`
	}{
		Key:        key,
		PlayerKeys: keys,
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("%v/api/players/", host), "application/json", bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("error performing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("error creating player key: %v", err)
	}
	return nil
}

func handleDelete(key string) error {
	requestURL := fmt.Sprintf("%v/api/players/%v", host, key)
	req, err := http.NewRequest("DELETE", requestURL, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error performing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading response body: %v", err)
		}
		return fmt.Errorf("error: %v", strings.TrimSpace(string(body)))
	}
	return nil
}

func handleAction(action, value string) error {
	if value != "" {
		switch action {
		case "setTime", "setVolume":
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			return sendPlayerAction(f)

		case "setVolumeMute":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return err
			}
			return sendPlayerAction(b)
		}
	}
	return sendPlayerAction(nil)
}

type PlayerKeys struct {
	Keys []string `json:"keys"`
}

func getPlayerKeys() (*PlayerKeys, error) {
	resp, err := http.Get(fmt.Sprintf("%v/api/players/", host))
	if err != nil {
		return nil, fmt.Errorf("error performing request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error: %v", string(body))
	}

	var keys PlayerKeys
	err = json.Unmarshal(body, &keys)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %v", err)
	}
	return &keys, nil
}

type Player struct {
	Key        string   `json:"key"`
	PlayerKeys []string `json:"playerKeys"`
}

func getPlayer(key string) (*Player, error) {
	resp, err := http.Get(fmt.Sprintf("%v/api/players/%v", host, key))
	if err != nil {
		return nil, fmt.Errorf("error performing request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	var data Player
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %v", err)
	}
	return &data, nil
}

func sendPlayerAction(value interface{}) error {
	data := struct {
		Action string      `json:"action"`
		Value  interface{} `json:"value,omitempty"`
	}{
		Action: action,
		Value:  value,
	}

	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshalling JSON request body: %v", err)
	}
	requestURL := fmt.Sprintf("%v/api/players/%v", host, key)
	req, err := http.NewRequest("PUT", requestURL, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error performing request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading response body: %v", err)
		}
		return fmt.Errorf("error: %v", strings.TrimSpace(string(body)))
	}
	return nil
}
