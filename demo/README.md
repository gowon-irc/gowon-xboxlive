# Bot demo

These files can be used to setup a test instance for demo or testing purposes using skaffold.

To run an ircd and bot instance in a kubernetes cluster (like k3d) cd to this directory and run:

    skaffold dev --tail

An [tiny](https://github.com/osa1/tiny) deployment is included. To use it run `tiny.sh`.
