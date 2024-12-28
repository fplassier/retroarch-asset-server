# retroarch-asset-server

Asset server for retroarch

## Compilation
You need Go 1.19 (minimum). Simply build your application by issuing
`go build`.

## Usage
Usage of retroarch-asset-server:
- **\--listen value**: Server listening address
- **\--frontend string**: path of the directory where frontend is stored
- **\--rom string**: path of the directory where ROMs are stored
- **\--system string**: path of the directory where systems are stored

When a location option is omitted, the server redirects to http://buildbot.libretro.com/assets/