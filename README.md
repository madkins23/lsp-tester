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
Use `<ctrl>-C` to kill it when you're done.
There is an hour timeout hard-coded in at the moment.

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

#### Default Format

Console output provides a line per message by default.

For example:
```shell
lsp-tester -console -clientPort=8006 -request=<file path>
```
might result in the following:
```
16:13:30 INF LSP starting
16:13:30 INF Send !=tester-->server #size=123 msg={"id":81,"jsonrpc":"2.0","method":"$/alive/eval","params":{"package":"cl-user","storeResult":true,"text":"(+ 2 (/ 15 5))"}} source=file
16:13:30 INF Rcvd !=tester<--server #size=58 msg={"jsonrpc":"2.0","method":"$/alive/refresh","params":{}}
16:13:30 INF Rcvd !=tester<--server #size=47 msg={"id":81,"jsonrpc":"2.0","result":{"text":"5"}}
16:13:30 INF Rcvd !=tester<--server #size=58 msg={"jsonrpc":"2.0","method":"$/alive/refresh","params":{}}
```

#### Expanded Format

The JSON content of the `msg` field can also be expanded using:
```shell
lsp-tester -console -expand -clientPort=8006 -request=<file path>
```
so that the previous log data would show as:
```
16:14:24 INF LSP starting
16:14:24 INF Send !=tester-->server #size=123 source=file
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
16:14:24 INF Rcvd !=tester<--server #size=58
{
  "jsonrpc": "2.0",
  "method": "$/alive/refresh",
  "params": {}
}
16:14:24 INF Rcvd !=tester<--server #size=47
{
  "id": 81,
  "jsonrpc": "2.0",
  "result": {
    "text": "5"
  }
}
16:14:24 INF Rcvd !=tester<--server #size=58
{
  "jsonrpc": "2.0",
  "method": "$/alive/refresh",
  "params": {}
}
```

#### Simple Format

On the other hand, large amounts of data can sometimes be generated
(especially during initialization) so there is a log simplification mode:
```shell
lsp-tester -console -simple -clientPort=8006 -request=<file path>
```
in which the previous log data would show as:
```
07:30:48 INF LSP starting
07:30:48 INF Receiver starting to=server
07:30:48 DBG Connected to=server
07:30:48 INF Send !=tester-->server #size=125 $Type=request %ID=81 %method=$/alive/eval <package=cl-user <storeResult=true <text="(+ 2 (/ 15 5))"
07:30:48 INF Rcvd !=tester<--server #size=58 $Type=notification %method=$/alive/refresh
07:30:48 INF Rcvd !=tester<--server #size=49 $Type=response %ID=81 <>method=$/alive/eval <>package=cl-user <>storeResult=true <>text="(+ 2 (/ 15 5))" >text=5
07:30:48 INF Rcvd !=tester<--server #size=58 $Type=notification %method=$/alive/refresh
```
This mode attempts to pull out key fields and only show small blocks of meaningful data.
Specific conventions used in this format:

| Example             | Definition                                     |
|---------------------|------------------------------------------------|
| `!=tester-->server` | Direction of message [^1]                      |
| `#size=125`         | Size of content from message header            |
| `$Type=request`     | Type of message [^2]                           |
| `%ID=81`            | Message ID                                     |
| `%method=$/alive/eval` | Method for request                             |
| `<text="(+ 2 (/ 15 5))` | Parameter with name prefixed by `<`            |
| `>text=5` | Result item with name prefixed by `>`           |
| `<>method=$/alive/eval` | Method from request provided with response [^3] |
| `<>text="(+ 2 (/ 15 5))"` | Parameter from request provided with response [^3] |                                 

Notes:

[^1]: In Nexus mode there will be two log items for each message.
The first will be from the client or server to the tester,
the second will be from the tester to the server or client as appropriate

[^2]: The `$Type` of message is derived from the available fields.
There is no specific "type" field in the Language Server Protocol.

[^3]: Method and parameter data  from requests is stored by ID,
looked up when a response message is found with the same ID, and
added to the log entry for the response using the `<>` prefix.
This data is not actually in the response message.

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

## Web Server

An embedded web server provides some interactive control over `lsp-tester`.
The following functionality may be invoked while the tester is running:
* Change the log format.
* Send messages stored in files to server or client.

### Starting the Web Server

The web server is only started if a `-webPort` flag is specified with a non-zero value:
```
lsp-tester -console -clientPort=8006 -serverPort=8007 -webPort=8008
```

The server will be accessible from a browser at `http://localhost:<webPort>`.
The main (and currently only) page:

![lsp-tester main web page](./images/webMain.png)

#### Connections

All current connections are displayed.
There can only be a single `server` connection,
but there can be multiple numbered `client-#` connections over time
(and theoretically at the same time).

#### Messaging

Messaging requires a directory of `.json` message files.
The `-messages` flag specifies the path to this directory:
```
lsp-tester -console -clientPort=8006 -serverPort=8007 -webPort=8008 -messages=<dirPath>
```
The `-messages` flag is only used when `-webPort` is used to activate the web server.
Message files are `.json` files with properly configured LSP messages.

On the main web page set the target for the message via the provided drop-down
which will have an entry for each current connection.
Use the message drop-down to set the message to be sent.
The `Send Message` button will send the actual message.

#### Change Log Format

The log format can be changed while `lsp-tester` is running.
The three radio buttons represent the three log formats as described above  in the **Output** section.
Select one of the log formats and use the `Change Log Format` message.
All subsequent messaging will be in the new format unless changed again.

### Output

Output from `lsp-tester` will continue to be via either `stderr` or a file.
There is currently no provision for seeing the log via the web interface
which is used only to control `lsp-tester`.

### Usage Scenario

The web interface provides the means to execute complex testing scenarios:

1. Bring up the LSP server with a specified port.
2. Run `lsp-tester` specifying the LSP server port in `-clientPort` and `--simple`.
3. Configure the VSCode extension to contact the `lsp-tester` `-serverPort`.
4. Start the extension or restart it via the **Developer: Reload Window** command.
5. Wait for the initialization traffic to clear in the `lsp-tester` output stream.
6. Use the web interface to change the output format to `default` or `expanded`.
7. Use the web interface to send any desired message to either the extension or the LSP server.

This scenario handles two problems:

1. The initialization traffic between the extension and the LSP server can be large.
2. It is useful to be able to send messages in both directions once the connection has started.

## Command Line Flags

| Flag          | Type     | Description                                          |
|---------------|----------|------------------------------------------------------|
| `-clientPort` | `uint`   | Client port number                                   |
| `-serverPort` | `uint`   | Server port number                                   |
| `-console`    | `bool`   | Log to the console instead of the specified log file |
| `-expand`     | `bool`   | Expand message JSON in log if true                   |
| `-simple`     | `bool`   | Show simplified console log entries                  |
| `-host`       | `string` | Host address (default "127.0.0.1")                   |
| `-logFile`    | `string` | Log file path (default "/tmp/lsp-tester.log")        |
| `-logJSON`    | `bool`   | Log output to file as JSON objects                   |
| `-request`    | `string` | Path to requestPath file (client mode)               |
| `-webPort`    | `uint`   | Port for web server for interactive control          |
| `-messages`   | `string` | Path to directory of message files (for Web server)  |
| `-help`       | `bool`   | Show usage and flags                                 |

Boolean (`bool`) flags do not require a value.
For example, the flag `-console` is equivalent to `-console=true`.
