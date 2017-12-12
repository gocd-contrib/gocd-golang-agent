GoLang agent for GoCD
=========================

[![Join the chat at https://gitter.im/gocd/gocd-golang-agent](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/gocd/gocd-golang-agent?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

GOCD agent golang implementation. Comparing to java implementation, golang agent has less installation dependency, less memory footprint and shorter boostrap time. More suitable for running in container.

Golang agent is based on "BuildCommand API" proposed [here](https://github.com/gocd/gocd/issues/1954). It's still working in progress. If you want to try out the golang agent, please build GoCD server from the latest master branch.

### Features not supported yet
* SCM materials other than git
* Java task plugins
* Java scm plugins such as Github-PR


### Installation

On Ubuntu:
```
# Add Bintray's GPG key:
sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys 379CE192D401AB61 
# Add repo
sudo echo deb https://dl.bintray.com/alex-hal9000/gocd-golang-agent master main | sudo tee -a /etc/apt/sources.list
sudo apt-get update
# Install the package  (add '-y --force-yes' after 'install' if automating)
sudo apt-get install gocd-golang-agent
```

### Configure Agent

Agent is designed to be configured by environment variables. The followings are available options:

* **GOCD_SERVER_URL**: Go server url, default to https://localhost:8154/go.
* **GOCD_AGENT_WORKING_DIR**: Agent working directory, default to Agent script launch directory. All build data will be inside this directory.
* **GOCD_AGENT_CONFIG_DIR**: Agent configurations for connecting to Go server, default to be "config" directory inside **GOCD_AGENT_WORKING_DIR** directory
* **GOCD_AGENT_LOG_DIR**: Agent log directory, without this configuration, log will be output to stdout.
* **DEBUG**: set this environment variable to any value will turn on debug log.


### Development

Check out source
* git clone https://github.com/gocd-contrib/gocd-golang-agent.git src/github.com/gocd-contrib/gocd-golang-agent

Building the agent binary
* go run src/github.com/gocd-contrib/gocd-golang-agent/build/build.go

### Download
Pre-build binary can be found here : https://bintray.com/gocd-contrib

## Contributing

Bug reports and pull requests are welcome on GitHub at https://github.com/gocd-contrib/gocd-golang-agent.

[Document for Developer](https://github.com/gocd-contrib/gocd-golang-agent/wiki/For-Developer)

