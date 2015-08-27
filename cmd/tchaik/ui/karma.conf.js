module.exports = function(config) {
  config.set({
    frameworks: ["mocha", "sinon-chai"],
    preprocessors: {
      "**/*.js": ["webpack"],
    },
    webpack: require("./webpack.tests.config.js"),
    reporters: ["mocha"],
    mochaReporter: {
      output: "autowatch",
    },
  });
};
