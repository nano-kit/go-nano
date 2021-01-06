package main

import (
	"log"
	"net/http"
	"os"

	"github.com/aclisp/go-nano"
	"github.com/aclisp/go-nano/component"
	"github.com/aclisp/go-nano/examples/demo/tadpole/logic"
	"github.com/aclisp/go-nano/serialize/json"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()

	app.Name = "tadpole"
	app.Author = "nano authors"
	app.Version = "0.0.1"
	app.Copyright = "nano authors reserved"
	app.Usage = "tadpole"

	// flags
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "addr",
			Value: ":23456",
			Usage: "game server address",
		},
	}

	app.Action = serve

	app.Run(os.Args)
}

func serve(ctx *cli.Context) error {
	components := &component.Components{}
	components.Register(logic.NewManager())
	components.Register(logic.NewWorld())

	// register all service
	options := []nano.Option{
		nano.WithIsWebsocket(true),
		nano.WithComponents(components),
		nano.WithSerializer(json.NewSerializer()),
		nano.WithCheckOriginFunc(func(_ *http.Request) bool { return true }),
		nano.WithHTTPHandler("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static")))),
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	addr := ctx.String("addr")
	nano.Listen(addr, options...)
	return nil
}
