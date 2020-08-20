senpai(1)

# NAME

senpai - your everyday IRC student

# SYNOPSIS

*senpai* [options...]

# OPTIONS

*-config* <path>
	Use a different path for the configuration file.

*-debug*
	Show all IRC messages that are received from/sent to the server.

# DESCRIPTION

senpai is an IRC client made for bouncers.  It supports the newest IRC
extensions, such as:

- _CHATHISTORY_, senpai fetches history from the server instead of keeping logs,
- _@+typing_, senpai shows when others are typing a message,
- and more to come!

# CONFIGURATION

senpai needs a configuration file to start.  It searches for it in the following
location:

	$XDG_CONFIG_HOME/senpai/senpai.yaml

If unset, $XDG_CONFIG_HOME defaults to *~/.config*.

For information about the configuration format, see *senpai*(5).

# USER INTERFACE

The user interface of senpai consists of 4 parts.  Starting from the bottom:

The *buffer list*, shows joined channels.  The special buffer *home* is where
private messages and server notices are shown.

On the row above, *input field* is where you type in messages or commands (see
*COMMANDS*).  By default, when you type a message, senpai will send a
notification to the current channel and other will be able to know you are
typing.

On the row above, the *status line* (or... just a line if nothing is
happening...) is where typing indicator are shown (e.g. "dan- is typing...").

Finally, the *timeline* is displayed on the rest of the screen.  There are
several types of messages in the timeline:

- User messages are shown with their nicknames,
- User actions (*/me*) are shown with an asterisk (*\**) followed by the user
  nickname,
- Status messages, such as joins, parts, topics and name lists, are shown with
  two dashes (*--*),
- Notices are shown with an asterisk (*\**) followed by the message in
  parenthesis.

# KEYBOARD SHORTCUTS

*CTRL-C*
	Quit the program.

*CTRL-U*, *PgUp*
	Go up in the timeline.

*CTRL-D*, *PgDown*
	Go down in the timeline.

*CTRL-N*, *ALT-RIGHT*
	Go to the next buffer.

*CTRL-P*, *ALT-LEFT*
	Go to the previous buffer.

*UP*, *DOWN*, *LEFT*, *RIGHT*, *HOME*, *END*, *BACKSPACE*, *DELETE*
	Edit the text in the input field.

*ENTER*
	Sends the contents of the input field.

*TAB*
	Trigger the auto-completion.  Press several times to cycle through
	completions.

*CTRL-L*
	Refresh the window.

# COMMANDS

If the user types and sends a message that starts with a slash (*/*), it will
instead be interpreted as a command:

	/_name_ argument1 argument2...

_name_ is matched case-insensitively.  It can be one of the following:

*J*, *JOIN* <channel>
	Joins the given channel.

*PART* [channel]
	Part the given channel, defaults to the current one if omitted.

*NAMES*
	Shows the member list of the current channel.  Powerlevels (such as _@_ for
	"operator", or _+_ for "voice") are shown in green.

*TOPIC* [topic]
	If _topic_ is omitted, shows the topic of the current channel and, if
	available, the person who set it and the time when it has been set.

	Otherwise, change the topic of the current channel to _topic_.

*MSG* <target> <content>
	Sends _content_ to _target_.

*R* <content>
	Reply to the last person who sent you a private message.

*ME* <content>
	Sends a message prefixed with your nick (a user action).

*QUOTE* <raw message>
	Sends _raw message_ verbatim.

# SEE ALSO

*senpai*(5)

# AUTHORS

Your home-made senpai by Hubert Hirtz <hubert@hirtzfr.eu>!++
Sources are available at <https://git.sr.ht/~taiite/senpai>.++
Patches are welcome by email at <~taiite/public-inbox@lists.sr.ht>.
