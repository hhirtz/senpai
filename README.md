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
real: Senpai is the best senpai
user: sasluser
password: "my password can't be this cute"
EOF
go run ./cmd/irc
```

Then, type `/join #senpai` and have a chat!

See `doc/senpai.1.scd` for more information and `doc/senpai.5.scd` for more
configuration options!

## Contributing and stuff

Patches are welcome at <mailto:~taiite/public-inbox@lists.sr.ht>!

Browse tickets at <https://todo.sr.ht/~taiite/senpai>.

## License

This senpai is open source! Please use it under the ISC license.

Copyright (C) 2020 Hubert Hirtz
