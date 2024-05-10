package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name: "nigiri",
		Commands: []*cli.Command{
			{
				Name:  "build",
				Usage: "TODO: Write usage",
				Action: func(c *cli.Context) error {
					log.Println("build")
					return nil
				},
			}, {
				Name:  "run",
				Usage: "TODO: Write usage",
				Action: func(c *cli.Context) error {
					log.Println("run")
					return nil
				},
			}, {
				Name:    "remove",
				Aliases: []string{"rm"},
				Usage:   "TODO: Write usage",
				Action: func(c *cli.Context) error {
					log.Println("rm")
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
