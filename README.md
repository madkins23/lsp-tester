# lsp-tester
Tool to do basic testing of the communication between a VSCode language extension and
a Language Server Protocol (LSP) server providing support to that extension.

![GitHub release (latest by date)](https://img.shields.io/github/v/release/madkins23/lsp-tester)
[![Go Reference](https://pkg.go.dev/badge/github.com/madkins23/lsp-tester.svg)](https://pkg.go.dev/github.com/madkins23/lsp-tester)
![GitHub](https://img.shields.io/github/license/madkins23/lsp-tester)
[![Go Report Card](https://goreportcard.com/badge/github.com/madkins23/lsp-tester)](https://goreportcard.com/report/github.com/madkins23/lsp-tester)

## Notes

This tool was created to provide a view into message traffic between a VSCode extension and
the LSP server used by the extension.
The author spent some time looking for a pre-existing tool of this sort.
The only thing found appeared to be out of date and not supported.
The author is resigned to finding out tomorrow that a much better tool already exists.
[So it goes](https://en.wikipedia.org/wiki/Slaughterhouse-Five).

## Modes

The `lsp-tester` application will connect to either a LSP server or a LSP client or both.
The two most useful modes are:

* Client mode allows (minimal) direct testing of the LSP without using VSCode.
  This can rule out issues that are directly related to the VSCode Language Extension code
  in the plugin.
* Nexus mode where `lsp-tester` sits between the VSCode plugin and the LSP.
  This will capture and log all traffic between the two in various [Log Formats](#log-formats).

In many cases the mode can be guessed by `lsp-tester` based on various flag settings.
There are some situations where this is insufficient and the `-mode` flag must be provided.
In these situations an error message will be shown and `lsp-tester` will exit:

```shell
$ lsp-tester -command=dummy
10:34:09 INF LSP starting
10:34:09 ERR Validate flags error="check --mode: can't guess -mode"
10:34:09 INF LSP finished
```

This can be fixed by explicitly specifying the `-mode`:

```shell
$ lsp-tester -mode=client -command=dummy
10:35:44 INF LSP starting
10:35:44 INF Receive starting to=server
```

### Client

Force client mode with flag `-mode=client`.

As a LSP client `lsp-tester` will connect to an LSP server.
A single request packet can be read from a JSON file and sent to the LSP server.
All traffic between `lsp-tester` and the LSP server is logged.

Example:
```shell
lsp-tester -serverPort=8006 -request=<file path>
```

This is a nice way to test single requests without using VSCode.
It is also a good way to verify that an unknown LSP actually runs.

When the connection protocol requires "client" and "server" flags
client mode requires configuration of the "server" flag (e.g. `--serverPort`).
This may seem confusing, but client mode means that `lsp-tester` is
acting as a client and connecting to the LSP server.

### Server

Force client mode with flag `-mode=server`.

As an LSP server `lsp-tester` will accept requests from a VSCode client
but there is at the current time no mechanism for responding.

Example:
```shell
lsp-tester -serverPort=8006
```

About all this will show is whatever startup message(s) is(are) sent when the
VSCode extension tries to connect to its REPL Server.
This is the least useful operating mode for `lsp-tester`.

When the connection protocol requires "client" and "server" flags
server mode requires configuration of the "client" flag (e.g. `--clientPort`).
This may seem confusing, but server mode means that `lsp-tester` is
acting as a server and connecting to the plugin client
(or providing a way for the plugin to connect to `lsp-tester` as a client).

### Nexus

Force client mode with flag `-mode=nexus`.

In this mode `lsp-tester` acts as both client and server,
passing LSP messages back and forth and logging them.

Example:
```shell
lsp-tester -serverPort=8006 -clientPort=8007
```
This is potentially very useful for debugging or testing
as `lsp-tester` will show all message traffic between the two.
See the **Usage Examples** section below for suggestions on how to use this mode.

When the connection protocol requires "client" and "server" flags
nexus mode requires configuration of both flags.
The "client" flag (e.g. `-clientPort`) connects to the VSCode plugin and
the "server" flag (e.g. `-serverPort') connects to the LSP server.

## Protocols

There are multiple communication protocols for VSCode to connect to an LSP.
A given LSP will (likely) support only a single protocol.
At the current time `lsp-tester` only supports:

* `TCP` protocol uses a TCP port to communicate.
* `Sub` protocol launches the LSP as a sub-process and communicates using its
  standard input and output.[^1]

There are other protocols that `lsp-tester` doesn't support at this time.
The two that are supported are known to the programmer,
so coding and testing for these protocols is possible.

[^1]: Standard error may also be used but is not supported by `lsp-tester` at this time.

### TCP

Force TCP protocol with flag `-protocol=tcp`.

In normal operation the VSCode plugin will launch the LSP,
then make a connection to a known TCP port connected to that LSP.
TCP connections are two-way so a single connection provides communication
from the plugin to the LSP and vice versa.

When running `lsp-tester` with this protocol use the server flag (e.g. `-serverPort`)
to connect  to the LSP and/or the client flag (e.g. `-clientPort`)
to provide a port for the plugin to connect to `lsp-tester`.

When using this protocol with `lsp-tester` in Server and Nexus modes
run `lsp-tester` separately from VSCode so that the plugin can connect.
The VSCode plugin should have some settings to determine the host and port.

### Sub

Force `Sub` protocol with flag `-protocol=sub`.

In normal operation the VSCode plugin will launch the LSP
with pipes connecting to the LSP's standard input and standard output.
These pipes are one way so there must be two.

When running `lsp-tester` with this protocol set the command to run the LSP
using the `-command` flag, which must contain any necessary arguments.
Since `lsp-tester` must launch the LSP it is not necessary to run it separately.
With this protocol it is necessary to use the `-mode` flag.

When using this protocol with `lsp-tester` in Server or Nexus modes
allow the VSCode plugin to launch `lsp-tester` with appropriate flags.
The VSCode plugin should have some settings to determine the LSP command and arguments.
In addition, set `-logLevel=error` and configure a `-logFile`
as the normal logging output would conflict with the communication protocol.
To simplify configuration of `lsp-tester` in plugin settings it is handy
to use a configuration file as described below in the section on
[Command Line Flags](#command-line-flags).

## Output

Log output is written to the console and optionally to a log file.
Logging can be configured in a number of ways.

### Log Levels

Logging is done using [`zerolog`](https://github.com/rs/zerolog) which provides
a variety of logging levels.
The level desired can be set using the `-logLevel=<level>` flag.
Levels used in `lsp-tester` are `error`, `warn`, `info`, `debug`, and `trace`.
Logging level `none` can be used to disable logging completely.

The default level is `info`.

### Log Streams

#### Console Output

Console output is always written to the standard output stream (`os.Stderr`).
Use `-logLevel=none` to turn this output off completely,
or choose an appropriate log format to reduce the amount of text.
Use the `-logFormat=<format>` flag to change the log format for the console.

When setting `-logLevel=none` an attempt is made to create a backup error log.
The log file `lsp-tester.err` will be created in the platform's "temporary" directory.
For Linux this will be `/tmp/lsp-tester.err`.
Other platforms may not have a temporary directory, in which case this file will not be created.
The log level of the created file is 'warn'.

The flag `-logLevel=none` may be useful when configuring Nexus mode for the Command protocol.

#### File Output

By default `lsp-tester` does not send output to a file.
Configure file output by specifying the `-logFile=<logFilePath>` flag:
```shell
lsp-tester -mode=nexus -command=dummy -logFile=<logFilePath>
```

Use the `-fileFormat=<format>` flag to change the log format for the log file.

By default, each invocation of `lsp-tester` with a log file defined
will cause the old log file to be truncated on open.
To cause new messages to be appended to a pre-existing log file use flag `-fileAppend`:
```shell
lsp-tester -serverPort=8006 -clientPort=8007 -logFile=<logFilePath> -fileAppend
```
In this case a blank line will be emitted to separate the new messages from the old ones:
```
09:39:52 INF Send !=server<--tester #size=125 msg={"id":"81","jsonrpc":"2.0","method":"$/alive/eval","params":{"package":"cl-user","storeResult":true,"text":"(+ 2 (/ 15 5))"}}
09:39:52 INF Rcvd !=server-->tester #size=58 msg={"jsonrpc":"2.0","method":"$/alive/refresh","params":{}}
09:39:52 INF Rcvd !=server-->tester #size=49 msg={"id":"81","jsonrpc":"2.0","result":{"text":"5"}}
09:39:52 INF Rcvd !=server-->tester #size=58 msg={"jsonrpc":"2.0","method":"$/alive/refresh","params":{}}

09:39:59 INF Send !=server<--tester #size=125 msg={"id":"81","jsonrpc":"2.0","method":"$/alive/eval","params":{"package":"cl-user","storeResult":true,"text":"(+ 2 (/ 15 5))"}}
09:39:59 INF Rcvd !=server-->tester #size=58 msg={"jsonrpc":"2.0","method":"$/alive/refresh","params":{}}
09:39:59 INF Rcvd !=server-->tester #size=49 msg={"id":"81","jsonrpc":"2.0","result":{"text":"5"}}
09:39:59 INF Rcvd !=server-->tester #size=58 msg={"jsonrpc":"2.0","method":"$/alive/refresh","params":{}}
```

### Log Formats

There are four logging formats which are available to both console and log files
and can be configured separately.
For either log destination the default format is `default`.

Specific conventions used in all output formats:

| Example             | Definition                          |
|---------------------|-------------------------------------|
| `!=server-->client` | Direction of message                |
| `#size=125`         | Size of content from message header |

In all formats but `json` these items will be at the left of every line
after the timestamp, log level, and message text.
In `json` mode these fields will still be present but not as easy to find.

The message direction is configured so that the `server`, when present, is on the left and
the `client`, when present, is on the right.
If either the client or the server is absent any messages will replace the missing
entity with `tester`, representing `lsp-tester` itself.

In the examples below the use of `logFormat=<format>` always implies
the same behavior for log files as by using `fileFormat=<format>`.
These are separate settings but work the same for the different log streams.

#### Format: `default`

Console output provides a line per message by default.

For example:
```shell
lsp-tester serverPort=8006 -request=<file path>
```
might result in the following:
```
16:13:30 INF LSP starting
16:13:30 INF Send !=server<--tester #size=123 msg={"id":81,"jsonrpc":"2.0","method":"$/alive/eval","params":{"package":"cl-user","storeResult":true,"text":"(+ 2 (/ 15 5))"}} source=file
16:13:30 INF Rcvd !=server-->tester #size=58 msg={"jsonrpc":"2.0","method":"$/alive/refresh","params":{}}
16:13:30 INF Rcvd !=server-->tester #size=47 msg={"id":81,"jsonrpc":"2.0","result":{"text":"5"}}
16:13:30 INF Rcvd !=server-->tester #size=58 msg={"jsonrpc":"2.0","method":"$/alive/refresh","params":{}}
```

Some of the lines can be very long.
These will likely wrap around in a terminal window.

This is the default format for output.

#### Format: `expand`

The JSON content of the `msg` field can also be expanded using:
```shell
lsp-tester -logFormat=expand -serverPort=8006 -request=<file path>
```
so that the previous log data would show as:
```
16:14:24 INF LSP starting
16:14:24 INF Send !=server<--tester #size=123 source=file
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
16:14:24 INF Rcvd !=server-->tester #size=58
{
  "jsonrpc": "2.0",
  "method": "$/alive/refresh",
  "params": {}
}
16:14:24 INF Rcvd !=server-->tester #size=47
{
  "id": 81,
  "jsonrpc": "2.0",
  "result": {
    "text": "5"
  }
}
16:14:24 INF Rcvd !=server-->tester #size=58
{
  "jsonrpc": "2.0",
  "method": "$/alive/refresh",
  "params": {}
}
```

#### Format `keyword`

On the other hand, large amounts of data can sometimes be generated
(especially during initialization) so there is a mode that attempts to
analyze the traffic and show the most useful bits:
```shell
lsp-tester -logFormat=keyword -serverPort=8006 -request=<file path>
```
in which the previous log data would show as:
```
07:30:48 INF LSP starting
07:30:48 INF Receiver starting to=server
07:30:48 INF Send !=server<--tester #size=125 $Type=request %ID=81 %method=$/alive/eval <package=cl-user <storeResult=true <text="(+ 2 (/ 15 5))"
07:30:48 INF Rcvd !=server-->tester #size=58 $Type=notification %method=$/alive/refresh
07:30:48 INF Rcvd !=server-->tester #size=49 $Type=response %ID=81 <>method=$/alive/eval <>package=cl-user <>storeResult=true <>text="(+ 2 (/ 15 5))" >text=5
07:30:48 INF Rcvd !=server-->tester #size=58 $Type=notification %method=$/alive/refresh
```
This mode attempts to pull out key fields and only show small blocks of meaningful data.
Specific conventions used in this format:

| Example                   | Definition                                         |
|---------------------------|----------------------------------------------------|
| `$Type=request`           | Type of message [^2]                               |
| `%ID=81`                  | Message ID                                         |
| `%method=initialize`      | Method for request                                 |
| `<text="(+ 2 (/ 15 5))`   | Parameter with name prefixed by `<`                |
| `>text=5`                 | Result item with name prefixed by `>`              |
| `<>method=$/alive/eval`   | Method from request provided with response [^3]    |
| `<>text="(+ 2 (/ 15 5))"` | Parameter from request provided with response [^3] |                                 

[^2]: The `$Type` of message is derived from the available fields.
There is no specific "type" field in the Language Server Protocol
so this derivation is somewhat fuzzy and may be wrong sometimes.

[^3]: Method and parameter data from requests is stored by ID,
looked up when a response message is found with the same ID, and
added to the log entry for the response using the `<>` prefix.
This data is not actually in the response message.

#### Format: `json`

It is also possible to generate the log statements as individual JSON records:
```shell
lsp-tester -logFormat=keyword -serverPort=8006 -request=<file path>
```
in which the previous log data would show as:
```
{"level":"info","time":"2023-05-06T17:25:54-07:00","message":"LSP starting"}
{"level":"debug","!from":"tester","!to":"server","#size":123,"msg":{"jsonrpc":"2.0","id":81,"method":"$/alive/eval","params":{"package":"cl-user","storeResult":true,"text":"(+ 2 (/ 15 5))"}},"time":"2023-05-06T17:25:54-07:00","message":"Send"}
{"level":"debug","&test":"client","!from":"server","!to":"tester","#size":58,"msg":{"jsonrpc":"2.0","method":"$\/alive\/refresh","params":{}},"time":"2023-05-06T17:25:54-07:00","message":"Received"}
{"level":"debug","&test":"client","!from":"server","!to":"tester","#size":47,"msg":{"id":81,"jsonrpc":"2.0","result":{"text":"5"}},"time":"2023-05-06T17:25:54-07:00","message":"Received"}
{"level":"debug","&test":"client","!from":"server","!to":"tester","#size":58,"msg":{"jsonrpc":"2.0","method":"$\/alive\/refresh","params":{}},"time":"2023-05-06T17:25:54-07:00","message":"Received"}
```

This format may be useful when logging to a file and post-processing the data.

## Web Server

An embedded web server provides some interactive control over `lsp-tester`.
The following functionality may be invoked while the tester is running:

* Change the log format for console or file output.
* Send messages stored in files to server or client.

### Starting the Web Server

The web server is only started if a `-webPort` flag is specified with a non-zero value:
```
lsp-tester -serverPort=8006 -clientPort=8007 -webPort=8008
```

The server will be accessible from a browser at `http://localhost:<webPort>`.
The main (and currently only) page:

![lsp-tester main web page](./images/webMain.png)

#### Icons

The generic "house" icon invokes the main page which is already displayed.
This is mostly handy for re-executing the page after restarting `lsp-tester`
or after invoking VSCode to cause the plugin to connect to `lsp-tester`.
In addition to displaying new connections it will clear the **Result** and **Errors** boxes.

The "bomb" icon executes a graceful shutdown of `lsp-tester`.

#### Connections

All current connections are displayed.
There can only be a single `server` connection,
but there can be multiple numbered `client-#` connections over time
(and theoretically at the same time).

#### Messaging

Messaging requires a directory of `.json` message files.
The `-messages` flag specifies the path to this directory:
```
lsp-tester -serverPort=8006 -clientPort=8007 -webPort=8008 -messages=<dirPath>
```

Message files are `.json` files with properly configured LSP messages.

On the main web page set the target for the message via the provided drop-down
which will have an entry for each current connection.
Use the message drop-down to set the message to be sent.
The `Send Message` button will send the actual message.

#### Change Log Format

The log format can be changed while `lsp-tester` is running.
There are side-by-forms for **Console** and **File** output format
(the latter will only be displayed if a log file is configured using the `-logFile` flag).
For each form the four radio buttons represent the [log formats](#log-formats) described above.
Select one of the log formats and use the `Change Log Format` button.
All subsequent messaging will be in the new format until changed again.

### Output

Output from `lsp-tester` will continue to be to the console and optionally a log file.
There is currently no provision for seeing the log via the web interface,
which is used only to control `lsp-tester`.

### Usage Examples

These examples assuming the TCP protocol.
Make appropriate changes if using another protocol.

#### Client Mode with Request

This is the original use case for `lsp-tester`.

1. Bring up the LSP server with a specified port.
2. Run `lsp-tester` specifying:
   * the LSP server port in `-serverPort`,
   * a JSON request file in `-request`, and
   * whatever non-default `-logFormat` is desired

After connecting to the server the request will be sent to the server.
This a quick test that a particular LSP feature returns the expected traffic.

After the message traffic `lsp-tester` continues running,
mostly because it isn't possible to determine when the server response traffic is done.
Kill the program with `<ctrl>-C` (or via the web interface if it is configured).

#### Nexus Mode with File Output

Initialization traffic between the VSCode extension host and the LSP server can be quite large.
It is possible to have the best of both worlds by configuring the log file output:

1. Bring up the LSP server with a specified port.
2. Run `lsp-tester` specifying:
    * the LSP server port in `-serverPort`,
    * an appropriate port in `-clientPort`,
    * a log file in `-logFile=<logFilePath>`,
    * small console output like `-logFormat=keyword`, and
    * large file format like `fileFormat=expand` or `fileFormat=json`.
3. Configure the VSCode extension to contact the chosen `lsp-tester` `-clientPort`.
4. Start the extension or restart it via the **Developer: Reload Window** command.

Console logging will give the gist of the message traffic and
file logging will capture the full messages for later examination.

Hint: if available, a second shell window or tab running `tail -f <logFilePath>`
will show the expanded traffic in real time.

#### Nexus Mode with Web Interface

The web interface provides the means to execute complex testing scenarios:

1. Bring up the LSP server with a specified port.
2. Run `lsp-tester` specifying:
   * the LSP server port in `-serverPort`,
   * an appropriate port in `-clientPort`, and 
   * a small output like `-logFormat=keyword`.
3. Configure the VSCode extension to contact the chosen `lsp-tester` `-clientPort`.
4. Start the extension or restart it via the **Developer: Reload Window** command.
5. Wait for the initialization traffic to clear in the `lsp-tester` output stream.
6. Use the web interface to change the output format to `default` or `expand`.
7. Use the web interface to send any desired message to either the extension or the LSP server.

This scenario handles two problems:

1. The initialization traffic between the extension and the LSP server can be large.
2. It is potentially useful to be able to send messages in both directions 
   once the connection has started.

## Command Line Flags

### Config Files

Flags may be bundled into configuration files and referenced as `@<file-path>`
on the `lsp-tester` command line:

```
lsp-tester @~/lsp-tester-cfgs/nexus.json -logFormat=keyword
```

Multiple configuration files may be specified, with later ones overriding
any common settings that may have been specified by earlier ones.

Configuration file(s) serve to override flag defaults as specified in the next section.
After all configuration files have been processed any remaining flags are processed,
with missing flags taking the values from the last relevant configuration file
instead of the normal default values.
All configuration files are processed before any flags are parsed.

Configuration files must be specified as absolute paths or relative to the
user's home directory using the `~/` convention on systems that support it.

Configuration files may be created in one of two formats based on the filename extension:

#### `.cfg` 

Each line in the file has the format:
```
<flag> = <value>
```

where `<flag>` is the name of one of the flags described below and
`<value>` is to be assigned to that flag.
The `<flag>` name may optionally be prefixed by a hyphen as if it were an actual flag.
Whitespace and blank lines are not important.
There is no provision for comments.

Example:

```
serverPort=8006
  -clientPort =    8007
-messages=~/work/lisp/lsp-tester/msg
```

#### `.json`

The configuration file contains a one-level JSON map from string to string.
The keys in the map represent flags and may optionally be prefixed by a hyphen.

Example:

```
{
    "serverPort": "8006",
    "-clientPort": "8007",
    "-messages": "~/work/lisp/lsp-tester/msg"
}
```

### Flag Descriptions

| Flag           | Type     | Description                                          |
|----------------|----------|------------------------------------------------------|
| `-mode`        | `string` | Set operating mode                                   |
| `-protocol`    | `string` | Set LSP communications protcol                       |
| `-commnd`      | `string` | LSP server command in Command protocol               |
| `-host`        | `string` | LSP server host address (default `"127.0.0.1"`)      |
| `-clientPort`  | `uint`   | Port number served for extension client to contact   |
| `-serverPort`  | `uint`   | Port number on which to contact LSP server           |
| `-webPort`     | `uint`   | Port for web server for interactive control          |
| `-logLevel`    | `string` | Set the log level (see below)                        |
| `-logFormat`   | `string` | Format value for console output (see below)          |
| `-logMsgTwice` | `bool`   | Show each message twice with `tester` in the middle. |
| `-logFile`     | `string` | Log file path (default no log file)                  |
| `-fileAppend`  | `bool`   | Append to any pre-existing log file                  |
| `-fileFormat`  | `string` | Format value for log file (see below)                |
| `-fileLevel`   | `string` | Set the log file level (see below)                   |
| `-maxFieldLen` | `uint`   | Maximum length for displayed fields (default 32)     |
| `-request`     | `string` | Path to file to be sent when connected (client mode) |
| `-messages`    | `string` | Path to directory of message files (for Web server)  |
| `-version`     | `bool`   | Show version of application                          |
| `-help`        | `bool`   | Show usage and flags                                 |

Boolean flags (e.g. `-version` and `-help`) do not require a value.
The presence of such a flag indicates a value of `true`.

Provided `-messages` and `-request` paths and the `-logFile` directory
must be specified as absolute paths or relative to the
user's home directory using the `~/` convention on systems that support it.

If `-messages=<directory>` is set the value for `-request` may be given as
a path relative to the `<directory>`.

The `-command` specified should be on the user's `PATH`.
Absolute and relatives paths do not work at the current time.

Format values can be set separately for console output and optional log file.

| Value     | Description                                                    |
|-----------|----------------------------------------------------------------|
| `default` | Linear format with messages output as single-line JSON         |
| `expand`  | Linear format with message content appended as multi-line JSON |
| `keyword` | Linear format with messages parsed to key fields               |
| `json`    | JSON object with message content embedded as more JSON         |

It is not necessary to specify `default` for `-logFormat` or `-fileFormat`.

Log level applies to the specified log stream.
Choices are specified in the following table.

| Value   | Description                  |
|---------|------------------------------|
| `none`  | No logging                   |
| `error` | Messages specifying errors   |
| `warn`  | Messages specifying warnings |
| `info`  | Informational messages       |
| `debug` | Debugging messages           |
| `trace` | Trace messages               |

Each value includes itself and all messages above it in the table.
The default value is `info`.

The `logMsgTwice` flag converts
```
17:29:31 INF Send !=server<--client-1 #size=122 $Type=request %ID=8 %method=$/alive/eval <package=cl-user <storeResult=true <text="(+ 2 (/ 15 5))"
17:29:31 INF Send !=server-->client-1 #size=46 $Type=response %ID=8 <>method=$/alive/eval <>package=cl-user <>storeResult=true <>text="(+ 2 (/ 15 5))" >text=5
```
to
```
17:36:34 INF Rcvd !=tester<--client-1 #size=122 $Type=request %ID=8 %method=$/alive/eval <package=cl-user <storeResult=true <text="(+ 2 (/ 15 5))"
17:36:34 INF Send !=server<--tester #size=122 $Type=request %ID=8 %method=$/alive/eval <package=cl-user <storeResult=true <text="(+ 2 (/ 15 5))"
17:36:34 INF Rcvd !=server->tester #size=46 $Type=response %ID=8 <>method=$/alive/eval <>package=cl-user <>storeResult=true <>text="(+ 2 (/ 15 5))" >text=5
17:36:34 INF Send !=tester->client-1 #size=46 $Type=response %ID=8 <>method=$/alive/eval <>package=cl-user <>storeResult=true <>text="(+ 2 (/ 15 5))" >text=5
```
to show the role of `lsp-tester` in passing messages back and forth.
This may not be very useful except when demonstrating that `lsp-tester` is not functional.
