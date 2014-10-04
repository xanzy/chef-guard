Chef-Guard [![Build Status](https://travis-ci.org/xanzy/chef-guard.svg)](https://travis-ci.org/xanzy/chef-guard) [![Gobuild Download](https://img.shields.io/badge/gobuild-download-green.svg?style=flat)](http://gobuild.io/github.com/xanzy/chef-guard) [![Gowalker Docs](http://gowalker.org/api/v1/badge)](https://gowalker.org/github.com/xanzy/chef-guard)
==========

**_NOTE: Even while the code is considered to be stable, Chef-Guard is still in BETA! So there will be some rapid changes to the code until version 1.0.0 is released!_**

Chef-Guard is a feature rich [Chef](http://www.getchef.com) add-on that protects your Chef server from untested and uncommitted (i.e. potentially dangerous) cookbooks by running several validations and checks during the cookbook upload process. In addition Chef-Guard will also monitor, audit, save and email (including a diff with the actual change) all configuration changes and is even capable of validating certain changes before passing them through to Chef.

So installing Chef-Guard onto your Chef server(s) will give you a highly configurable component that enables you to configure and enforce a common [workflow](http://xanzy.io/projects/chef-guard/workflows) for all your colleagues working with Chef.

Technically you can think of Chef-Guard as an extremely smart reverse proxy server written in [Go](https://golang.org/) and located/installed right in between Nginx and the Chef Server (see the [Installation](http://xanzy.io/projects/chef-guard/installation/installation.html) section for more details). This means that Chef-Guard runs completely server-side and does not require **any** client-side changes! This gives you the freedom to use whatever tools you like (e.g. knife, berks, the webui) to work with your Chef server and Chef-Guard will make sure all these tools follow the same workflow.

## Quickstart

_Assuming enough Chef knowledge, it shouldn't take more than 30 minutes to get you started!_

* Read the Chef-Guard [documentation](http://xanzy.io/projects/chef-guard) explaining and describing what Chef-Guard is and how it works
* Assuming you already have a running Chef environment, walk through the Chef-Guard [prerequisites](http://xanzy.io/projects/chef-guard/installation/prerequisites.html)
* Your now ready to follow the actual [installation](http://xanzy.io/projects/chef-guard/installation/installation.html) which (if you prefer) can be done using a [cookbook](http://xanzy.io/projects/chef-guard/installation/installation.html#installation-using-a-cookbook) in just a few minutes

## Building

You don't need to build Chef-Guard yourself in order to use it. Pre-built binaries, instructions and a ready to use cookbook can all be found [here](http://xanzy.io/projects/chef-guard/installation/installation.html). If however you would like to contribute to Chef-Guard and/or just feel adventurous and want to build CHef-Guard yourself, please see the [contributing](https://github.com/xanzy/chef-guard/blob/master/contributing.md) documentation to get you started.

## Getting Help

_Please read the [docs](http://xanzy.io/projects/chef-guard) first!_

* If you have an issue: report it on the [issue tracker](https://github.com/xanzy/chef-guard/issues)
* If you have a question: visit the #chef-guard channel on irc.freenode.net

## Author

Sander van Harmelen (<sander@xanzy.io>)

## License

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at <http://www.apache.org/licenses/LICENSE-2.0>

