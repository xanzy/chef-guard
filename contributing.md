Building/Contributing
=====================

Just as a heads up, building the project is actually as easy as running `go build` (assuming familiarity with [Go](https://golang.org/) of course).

## Vendoring

All dependencies are vendored in using '[godep save -r](https://github.com/tools/godep)' so all imports are rewritten to their current, vendored location. I'm currently working on a tweak for goimports so it also supports using with vendored projects. More to come on that later on.

## Building Chef-Guard on Linux

I cannot make it any easier that just using this one command: `go build` in the source directory :)

## Building Chef-Guard on MacOSX
### Prerequisites

Be sure to install Go to be able to crosscompile:
```
$ brew install go --cross-compile-common
```

### Compilation

Provide an OS to compile for:
```
$ GOOS=linux go build
```

For more information and details you should really checkout the Chef-Guard [project page](http://xanzy.io/projects/chef-guard) at [Xanzy](http://xanzy.io) which contains just about all the info you need to get started...

## Contributing

  1. Fork the repository on Github
  2. Create a named feature branch
  3. Write your change
  4. Write tests for your change (if applicable)
  5. Run the tests, ensuring they all pass
  6. Submit a Pull Request

