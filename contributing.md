Building/Contributing
=====================

Just as a heads up, building the project is actually as easy as getting the dependencies and running `go build` (assuming familiarity with [Go](https://golang.org/) of course).

## Installing the dependencies

* go get github.com/gorilla/mux
* go get code.google.com/p/gcfg
* go get code.google.com/p/goauth2/oauth
* go get github.com/google/go-github/github
* go get github.com/marpaia/chef-golang
* go get github.com/marpaia/graphite-golang
* go get bitbucket.org/kardianos/osext

## Building Chef-Guard

I cannot make it any easier that just using this one command: `go build` in the source directory :) For more information and details you should really checkout the Chef-Guard [project page](http://xanzy.io/projects/chef-guard) at [Xanzy](http://xanzy.io) which contains just about all the info you need to get started...

## Contributing

  1. Fork the repository on Github
  2. Create a named feature branch
  3. Write your change
  4. Write tests for your change (if applicable)
  5. Run the tests, ensuring they all pass
  6. Submit a Pull Request

