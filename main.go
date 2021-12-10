package main

import (
	"log"
	"regexp"

	"github.com/Adikso/minecraft-log4j-honeypot/extractor"
	"github.com/Adikso/minecraft-log4j-honeypot/minecraft"
)

func Analyse(text string) {
	log.Printf("Testing text: %s\n", text)

	pattern := regexp.MustCompile(`\${jndi:(.*)}`)
	finder := extractor.NewFinder(pattern)

	injections := finder.FindInjections(text)
	for _, url := range injections {
		log.Printf("Fetching payload for: jndi:%s", url.String())

		files, err := extractor.FetchFromLdap(url)
		if err != nil {
			log.Printf("Failed to fetch class from %s", url)
			continue
		}

		for _, filename := range files {
			log.Printf("Saved payload to file %s\n", filename)
		}
	}
}

func main() {
	server := minecraft.NewServer(":25565")
	server.ChatMessageCallback = Analyse
	server.AcceptLoginCallback = Analyse

	if err := server.Run(); err != nil {
		log.Println(err)
	}
}
