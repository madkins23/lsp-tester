# lsp-tester
Tool to do basic testing of a Language Server Protocol (LSP) server.

[![Go Report Card](https://goreportcard.com/badge/github.com/madkins23/lsp-tester)](https://goreportcard.com/report/github.com/madkins23/lsp-tester)
![GitHub](https://img.shields.io/github/license/madkins23/lsp-tester)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/madkins23/lsp-tester)
[![Go Reference](https://pkg.go.dev/badge/github.com/madkins23/lsp-tester.svg)](https://pkg.go.dev/github.com/madkins23/lsp-tester)

## Notes

This is an early (pre-)release.
This `lsp-tester` tool was originally created to help with work on the `Alive` VSCode
language extension and the `Alive-lsp` LSP server.
All work was done on Linux.
_Your mileage may vary_.

There is no "off" switch for `lsp-tester`.
Use <ctrl>-C to kill it when you're done.
There is an hour timeout hard-coded in at the moment.
_Mea culpa_.

## Modes

The `lsp-tester` application will connect to either a LSP server or a LSP client or both.
LSP messages are logged to either the console or a file or both.

### Client

As a LSP client `lsp-tester` will connect to a running LSP specified by
a host address (which defaults to `127.0.0.1`) and client port number.
A single request packet can be read from a JSON file and sent to the LSP server.
All traffic between `lsp-tester` and the LSP server is logged.

Example:
```shell
lsp-tester -clientPort=8006 -request=<file path>
```
This is a nice way to test single requests without using VSCode.

### Server

As an LSP server `lsp-tester` will accept requests from a VSCode client
but there is at the current time no mechanism for responding.

Example:
```shell
lsp-tester -serverPort=8006
```
About all this will show is whatever startup request is made when the
VSCode extension tries to connect to its REPL Server.

If the extension code is able to re-connect after its server goes down
then it might be possible to use the real server to get the extension started
and then kill the server and bring up `lsp-tester` in its place.

### Nexus

In this mode `lsp-tester` acts as both client and server,
passing LSP messages back and forth and logging them.

Example:
```shell
lsp-tester -clientPort=8006 -serverPort=8007
```

This is potentially very useful for debugging or testing
as `lsp-tester` will show all message traffic.

## Output

Output can be directed to the console or a file.

### Console Output

Console output provides a line per message by default.

For example:
```shell
lsp-tester -console -clientPort=8006 -request=<file path>
```
might result in the following:
```
16:47:14 INF LSP starting
16:47:14 DBG Send !from=tester !to=server #size=187 msg={"id":81,"jsonrpc":"2.0","method":"$/alive/loadFile","params":{"path":"/home/marc/work/go/src/github.com/madkins23/lsp-tester/msgs/alive/simple.lisp","showStderr":true,"showStdout":true}}
16:47:14 DBG Received !from=server !to=tester #size=58 &test=client msg={"jsonrpc":"2.0","method":"$/alive/refresh","params":{}}
16:47:14 DBG Received !from=server !to=tester #size=210 &test=client msg={"jsonrpc":"2.0","method":"$/alive/stdout","params":{"data":"; compiling file \"/home/marc/work/go/src/github.com/madkins23/lsp-tester/msgs/alive/simple.lisp\" (written 05 MAY 2023 04:54:19 PM):"}}
16:47:14 DBG Received !from=server !to=tester #size=162 &test=client msg={"jsonrpc":"2.0","method":"$/alive/stdout","params":{"data":"; wrote /home/marc/work/go/src/github.com/madkins23/lsp-tester/msgs/alive/simple.fasl"}}
16:47:14 DBG Received !from=server !to=tester #size=103 &test=client msg={"jsonrpc":"2.0","method":"$/alive/stdout","params":{"data":"; compilation finished in 0:00:00.004"}}
16:47:14 DBG Received !from=server !to=tester #size=71 &test=client msg={"jsonrpc":"2.0","method":"$/alive/stdout","params":{"data":"5040 "}}
16:47:14 DBG Received !from=server !to=tester #size=83 &test=client msg={"jsonrpc":"2.0","method":"$/alive/stdout","params":{"data":"\"Hello World!\" "}}
16:47:14 DBG Received !from=server !to=tester #size=58 &test=client msg={"jsonrpc":"2.0","method":"$/alive/refresh","params":{}}
```

The JSON content of the `msg` field can also be expanded using:
```shell
lsp-tester -console -expand -clientPort=8006 -request=<file path>
```
would show as:
```
16:51:03 INF LSP starting
16:51:03 DBG Send !from=tester !to=server #size=123
{
  "id": 81,
  "jsonrpc": "2.0",
  "method": "$/alive/eval",
  "params": {
    "package": "cl-user",
    "storeResult": true,
    "text": "(+ 2 (/ 15 5))"
  }
}
16:51:03 DBG Received !from=server !to=tester #size=58 &test=client
{
  "jsonrpc": "2.0",
  "method": "$/alive/refresh",
  "params": {}
}
16:51:03 DBG Received !from=server !to=tester #size=47 &test=client
{
  "id": 81,
  "jsonrpc": "2.0",
  "result": {
    "text": "5"
  }
}
16:51:03 DBG Received !from=server !to=tester #size=58 &test=client
{
  "jsonrpc": "2.0",
  "method": "$/alive/refresh",
  "params": {}
}
```

### File Output

By default `lsp-tester` will direct output to a file using the same
format described above.
File output is set by _not_ using the `-console` flag.
The default filename is `/tmp/lsp-tester.log` but can be overridden:
```shell
lsp-tester -clientPort=8006 -serverPort=8007 -logFile=<path>
```

File contents can also be dumped as JSON records:
```shell
lsp-tester -clientPort=8006 -serverPort=8007 -logFile=<path> -logJSON
```
yielding:
```
{"level":"info","time":"2023-05-06T17:25:54-07:00","message":"LSP starting"}
{"level":"debug","!from":"tester","!to":"server","#size":123,"msg":{"jsonrpc":"2.0","id":81,"method":"$/alive/eval","params":{"package":"cl-user","storeResult":true,"text":"(+ 2 (/ 15 5))"}},"time":"2023-05-06T17:25:54-07:00","message":"Send"}
{"level":"debug","&test":"client","!from":"server","!to":"tester","#size":58,"msg":{"jsonrpc":"2.0","method":"$\/alive\/refresh","params":{}},"time":"2023-05-06T17:25:54-07:00","message":"Received"}
{"level":"debug","&test":"client","!from":"server","!to":"tester","#size":47,"msg":{"id":81,"jsonrpc":"2.0","result":{"text":"5"}},"time":"2023-05-06T17:25:54-07:00","message":"Received"}
{"level":"debug","&test":"client","!from":"server","!to":"tester","#size":58,"msg":{"jsonrpc":"2.0","method":"$\/alive\/refresh","params":{}},"time":"2023-05-06T17:25:54-07:00","message":"Received"}
```

## Command Line Flags

| Flag           | Type      | Description                                           |
|----------------|-----------|-------------------------------------------------------|
| `-clientPort`  | `uint`    | Client port number                                    |
| `-console`     | `bool`    | Log to the console instead of the specified log file  |
| `-expand`      | `bool`    | Expand message JSON in log if true                                                      |
| `-host`        | `string`  |      Host address (default "127.0.0.1")                                                 |
| `-logFile`     | `string`  |  Log file path (default "/tmp/lsp-tester.log")                                                     |
| `-logJSON`     | `bool`    |    Log output to file as JSON objects                                                   |
| `-request`     | `string`  |   Path to requestPath file (client mode)                                                    |
| `-serverPort`  | `uint`    |  Server port number                                                     |

Boolean (`bool`) flags do not require a value.
The flag `-console` is equivalent to `-console=true`.
