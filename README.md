# mailwebadmin
mailwebadmin is a software used to administrate a mailserver (create accounts, delete accounts, administrate aliases, change passwords and so on). It requires a setup as described [here](https://workaround.org/ispmail/jessie), with one important difference: Currently only SHA-512 is supported and not SHA-256, but this is something that should be really easy to fix. If you want to use that feel free to contact me or write the code yourself and use a pull request.

The software is written in Go, but requires the Linux crypt(3). I've tested it on Debian and Ubuntu and it worked fine (sadly enough alpine does not work). It is also possible that it works under Windows, but I've never tested it. The wrapper can be found [here](https://github.com/amoghe/go-crypt).

The preferred way to install it is by using docker, there is a docker file (in this repository) and you can also pull directly from [Docker Hub](https://hub.docker.com/r/fabianwe/mailwebadmin/).

Information about the installation can be found on the [project Wiki](https://github.com/FabianWe/mailwebadmin/wiki), the source code documentation is also available on [GoDoc](https://godoc.org/github.com/FabianWe/mailwebadmin).

## Current Version
The current version is 1.0, it hasn't been properly tested, but it should work (though it would be nice if someone reviews it especially regarding security).

## License
The software is distributed under the MIT License, see the [license page in the wiki](https://github.com/FabianWe/mailwebadmin/wiki/License) for more information.
