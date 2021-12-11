package minecraft

import (
	"encoding/json"
	"log"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/google/uuid"
)

func (s *Server) acceptListPing(conn net.Conn) {
	var p pk.Packet
	for i := 0; i < 2; i++ { // ping or list. Only accept twice
		err := conn.ReadPacket(&p)
		if err != nil {
			return
		}

		switch p.ID {
		case 0x00: //List
			err = conn.WritePacket(pk.Marshal(0x00, pk.String(listResp())))
		case 0x01: //Ping
			err = conn.WritePacket(p)
		}
		if err != nil {
			return
		}
	}
}

type player struct {
	Name string    `json:"name"`
	ID   uuid.UUID `json:"id"`
}

// listResp return server status as JSON string
func listResp() string {
	var list struct {
		Version struct {
			Name     string `json:"name"`
			Protocol int    `json:"protocol"`
		} `json:"version"`
		Players struct {
			Max    int      `json:"max"`
			Online int      `json:"online"`
			Sample []player `json:"sample"`
		} `json:"players"`
		Description chat.Message `json:"description"`
		FavIcon     string       `json:"favicon,omitempty"`
	}

	list.Version.Name = "1.16.5"
	list.Version.Protocol = ProtocolVersion
	list.Players.Max = MaxPlayer
	list.Players.Online = 0
	list.Players.Sample = []player{}
	list.Description = chat.Message{Text: "A Minecraft Server"}

	data, err := json.Marshal(list)
	if err != nil {
		log.Panic("Marshal JSON for status checking fail")
	}
	return string(data)
}
