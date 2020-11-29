# NOTICE me :senpai!

Welcome home, desune~

![a screenshot of your senpai!](https://ellidri.org/senpai/screen.png)

Works best with soju!

## ... How?

```shell
mkdir -p ~/.config/senpai
cat <<EOF >~/.config/senpai/senpai.yaml
addr: irc.freenode.net:6697
nick: senpai
password: "my password can't be this cute"
EOF
go run ./cmd/irc
```

Then, type `/join #senpai` and have a chat!

See `doc/senpai.1.scd` for more information and `doc/senpai.5.scd` for more
configuration options!

## Debugging errors, testing servers

If you run into errors and want to find the WHY OH WHY, or if you'd like to try
things out with an IRC server, then you have two options:

1. Run senpai with the `-debug` argument (or put `debug: true`) in your config,
   it will then print in `home` all the data it sends and receives.
2. Run the test client, that uses the same IRC library but exposes a simpler
   interface, by running `go run ./cmd/test -help`.

## Contributing and stuff

Patches are welcome at <mailto:~taiite/public-inbox@lists.sr.ht>!

Browse tickets at <https://todo.sr.ht/~taiite/senpai>.

## License

This senpai is open source! Please use it under the ISC license.

Copyright (C) 2020 Hubert Hirtz
