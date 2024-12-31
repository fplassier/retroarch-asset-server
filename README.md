# retroarch-asset-server

Asset server for retroarch

## Compilation
You need Go 1.19 (minimum). Simply build your application by issuing `go build`. To build a statically linked executable, you can issue `go build -tags netgo` instead.

## Usage
```
retroarch-asset-server COMMAND [OPTIONS...]
```
Available commands are:
- **help**: print this help or the provided command help
- **version**: Print the application version.
- **serve**: Start the server (default command).

### help
```
retroarch-asset-server help [COMMAND_NAME]
```
Print the general help or, if a command name is provided, the help of this command. Then the program exits.

### version
```
retroarch-asset-server version
```
Print the retroarch-asset-server version then exit.

### serve
```
retroarch-asset-server serve [-listen ADDR] [-frontend PATH] [-system PATH] [-rom PATH]
```
Start serving the assets. When a location option is omitted, the server acts as a reverse proxy for http://buildbot.libretro.com/assets/

### Target specific commands
#### Windows
##### register-svc
```
retroarch-asset-server register-svc [-listen ADDR] [-frontend PATH] [-system PATH] [-rom PATH]
```
Register the current executable as an auto-starting Windows service. The options are the same that **serve** command ones.

##### unregister-svc
```
retroarch-asset-server unregister-svc
```
Unregister the retroarch-asset-server Windows service
