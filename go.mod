module git.sr.ht/~taiite/senpai

go 1.16

require (
	git.sr.ht/~emersion/go-scfg v0.0.0-20201019143924-142a8aa629fc
	github.com/gdamore/tcell/v2 v2.3.11
	github.com/mattn/go-runewidth v0.0.10
	golang.org/x/term v0.0.0-20201210144234-2321bbc49cbf
	golang.org/x/time v0.0.0-20210611083556-38a9dc6acbc6
	mvdan.cc/xurls/v2 v2.3.0
)

replace github.com/gdamore/tcell/v2 => github.com/hhirtz/tcell/v2 v2.3.12-0.20210807133752-5d743c3ab0c9
