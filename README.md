# OCC - OpenShift Command Center

OpenShift Command Center is a toolkit built to provide an ephemeral access environment to administer OpenShift Clusters.

OCC is based on the following three core principles:

* Isolation between multiple environments (multiple cluster connections at once)
* Isolation from any local environment interference
* Ephemeral kubeconfig and filesystem (deletion on close)

While not a _truly_ ephemeral environment, occ gives SREs the flexibility to do things like save logs to the filesystem in order to grep them, and then they're automatically removed once the container exits, preventing any accidental leaking of data unless the SRE explicitly copies them out of the container. 

---

# Contributing

When Contributing to occ, please keep the following practices in mind:

1. Use logging when not printing the output of the command
    1. When we use the logging library to print debug or other output, it's automatically written to stderr (and makes the output parsable by next processes) and the end-user can hide any log levels they don't want to see with the -v flag.
1. Use viper for any user-configurable flags or defaults
    1. This lets the end-user add things to their config file that otherwise would be flags they always want to run, or allows them to set multiple config files for separate scenarios, etc.  Viper also gives us automatic ENV var parsing for the flags as well, so the arg parsing order ends up being `viper defaults -> config file -> env vars -> arg flags`.


When contributing on MacOS, in order to build the binary you will also need the following package installed from brew:

```
brew install gpgme
```

When building on linux, make sure to install the podman build dependencies from [podman.io/.../build-dependencies](https://podman.io/getting-started/installation#build-and-run-dependencies).
