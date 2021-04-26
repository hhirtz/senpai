senpai(5)

# NAME

senpai - Configuration file format and settings

# DESCRIPTION

A senpai configuration file is a YAML file.

Some settings are required, the others are optional.

# SETTINGS

*addr* (required)
	The address (_host:port_) of the IRC server.  senpai only supports TLS
	connections and thus you must specify the TLS port of the server (in most
	cases, 6697 or 7000).

*nick* (required)
	Your nickname, sent with a _NICK_ IRC message. It mustn't contain spaces or
	colons (*:*).

*real*
	Your real name, or actually just a field that will be available to others
	and may contain spaces and colons.  Sent with the _USER_ IRC message.  By
	default, the value of *nick* is used.

*user*
	Your username, sent with the _USER_ IRC message and also used for SASL
	authentication.  By default, the value of *nick* is used.

*password*
	Your password, used for SASL authentication.

*highlights*
	A list of keywords that will trigger a notification and a display indicator
	when said by others.  By default, senpai will use your current nickname.

*on-highlight*
	A command to be executed via _sh_ when you are highlighted.  The following
	format specifiers are expanded with respect to the highlight:

[[ *Format specifier*
:< *Description*
|  %%
:  literal %
|  %b
:  buffer where the message appeared
|  %h
:  equals 1 if _%b_ is the current buffer, 0 otherwise
|  %m
:  content of the message
|  %n
:  nickname of the sender

*nick-column-width*
	The number of cell that the column for nicknames occupies in the timeline.
	By default, 16.

*chan-column-width*
	The number of cell that the column for channels occupies.  By default, 16.

*no-tls*
	Disable TLS encryption. By default, false.

*debug*
	Dump all sent and received data to the home buffer, useful for debugging.
	By default, false.

# EXAMPLES

A minimal configuration file to connect to freenode as "Guest123456":

```
addr: chat.freenode.net:6697
nick: Guest123456
```

A more advanced configuration file that enables SASL authentication, sends
notifications on highlight and decreases the width of the nick column to 12
(note: _swaymsg_ is specific to sway, a wayland compositor.  Use whatever you
need to know if the terminal emulator that runs senpai has focus):

```
addr: chat.freenode.net:6697
nick: Guest123456
user: senpai
real: Guest von Lenon
password: A secure password, I guess?
highlights:
	- guest
	- senpai
on-highlight: |
    FOCUS=$(swaymsg -t get_tree | jq '..|objects|select(.focused==true)|.name' | grep senpai | wc -l)
    if [ %h -eq 0 ] || [ $FOCUS -eq 0 ]; then
        notify-send "[%b] %n" "%m"
    fi
nick-column-width: 12
```

# SEE ALSO

*senpai*(1)
