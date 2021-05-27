senpai(5)

# NAME

senpai - Configuration file format and settings

# DESCRIPTION

A senpai configuration file is a YAML file.

Some settings are required, the others are optional.

# SETTINGS

*addr* (required)
	The address (_host[:port]_) of the IRC server. senpai uses TLS connections
	by default unless you specify *no-tls* option. TLS connections default to
	port 6697, plain-text use port 6667.

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
	environment variables are set with repect to the highlight, THEY MUST APPEAR
	QUOTED IN THE SETTING, OR YOU WILL BE OPEN TO ATTACKS.

[[ *Environment variable*
:< *Description*
|  BUFFER
:  buffer where the message appeared
|  HERE
:  equals 1 if _BUFFER_ is the current buffer, 0 otherwise
|  MESSAGE
:  content of the message
|  SENDER
:  nickname of the sender

	Note: when passing those to *notify-send*(1), some notification daemons use
	*\* for escape sequences in the body, which causes *\* to disappear from the
	message or triggers unintended side-effects (like newlines).

	To get around this, you can double the backslash with the following snippet:

```
on-highlight: |
    escape() {
        printf "%s" "$1" | sed 's#\\#\\\\#g'
    }
    notify-send "[$BUFFER] $SENDER" "$(escape "$MESSAGE")"
```

*nick-column-width*
	The number of cell that the column for nicknames occupies in the timeline.
	By default, 16.

*chan-column-width*
	The number of cell that the column for channels occupies.  By default, 16.

*no-tls*
	Disable TLS encryption.  Defaults to false.

*no-typings*
	Prevent senpai from sending typing notifications which let others know when
	you are typing a message.  Defaults to false.

*mouse*
	Enable or disable mouse support.  Defaults to true.

*colors*
	Settings for colors of different UI elements.

	Colors are represented as numbers from 0 to 255 for 256 default terminal
	colors respectively. -1 has special meaning of default terminal color. To
	use true colors, *#*_rrggbb_ notation is supported.

	Colors are set as sub-options of the main *colors* option:

```
colors:
    prompt: 3 # green
```

[[ *Sub-option*
:< *Description*
|  prompt
:  color for ">"-prompt that appears in command mode

*debug*
	Dump all sent and received data to the home buffer, useful for debugging.
	By default, false.

# EXAMPLES

A minimal configuration file to connect to Libera.Chat as "Guest123456":

```
addr: irc.libera.chat
nick: Guest123456
```

A more advanced configuration file that enables SASL authentication, sends
notifications on highlight and decreases the width of the nick column to 12
(note: _swaymsg_ is specific to sway, a wayland compositor.  Use whatever you
need to know if the terminal emulator that runs senpai has focus):

```
addr: irc.libera.chat
nick: Guest123456
user: senpai
real: Guest von Lenon
password: A secure password, I guess?
highlights:
	- guest
	- senpai
on-highlight: |
    escape() {
        printf "%s" "$1" | sed 's#\\#\\\\#g'
    }
    FOCUS=$(swaymsg -t get_tree | jq '..|objects|select(.focused==true)|.name' | grep senpai | wc -l)
    if [ "$HERE" -eq 0 ] || [ $FOCUS -eq 0 ]; then
        notify-send "[$BUFFER] $SENDER" "$(escape "$MESSAGE")"
    fi
nick-column-width: 12
```

# SEE ALSO

*senpai*(1)
