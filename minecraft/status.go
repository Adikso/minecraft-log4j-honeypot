package minecraft

import (
	"encoding/json"
	"log"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/google/uuid"
)

func (s *Session) acceptListPing(conn net.Conn) {
	var p pk.Packet
	for i := 0; i < 2; i++ { // ping or list. Only accept twice
		err := conn.ReadPacket(&p)
		if err != nil {
			return
		}

		switch p.ID {
		case 0x00: //List
			err = conn.WritePacket(pk.Marshal(0x00, pk.String(s.listResp())))
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
func (s *Session) listResp() string {
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

	list.Version.Name = "1.7.5"
	list.Version.Protocol = int(s.ProtocolVersion)

	version := s.GetVersionName()
	if version == "" {
		list.Version.Name = "1.16.5"
		list.Version.Protocol = 754
	} else {
		list.Version.Name = version
		list.Version.Protocol = int(s.ProtocolVersion)
	}

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

func (s *Session) GetVersionName() string {
	mapping := map[int32]string{
		4: "1.7.5",
		5: "1.7.10",
		47: "1.8.9",
		107: "1.9",
		108: "1.9.1",
		109: "1.9.2",
		110: "1.9.4",
		210: "1.10.2",
		315: "1.11",
		316: "1.11.2",
		335: "1.12",
		338: "1.12.1",
		340: "1.12.2",
		393: "1.13",
		401: "1.13.1",
		404: "1.13.2",
		477: "1.14",
		480: "1.14.1",
		485: "1.14.2",
		490: "1.14.3",
		498: "1.14.4",
		573: "1.15",
		575: "1.15.1",
		578: "1.15.2",
		735: "1.16",
		736: "1.16.1",
		751: "1.16.2",
		753: "1.16.3",
		754: "1.16.5",
	}

	if _, has := mapping[s.ProtocolVersion]; !has {
		return ""
	}

	return mapping[s.ProtocolVersion]
}
