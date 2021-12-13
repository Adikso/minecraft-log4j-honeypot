package minecraft

import (
	_ "embed"
	"fmt"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/nbt"
	"github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/offline"
	"github.com/google/uuid"
	"log"
	"strings"
)

const (
	MaxPlayer = 50
)

type Session struct {
	Server          *Server
	ProtocolVersion int32
}

type Server struct {
	Address             string
	AcceptLoginCallback func(userName string)
	ChatMessageCallback func(text string)
}

func NewServer(address string) *Server {
	return &Server{
		Address: address,
	}
}

func (s *Server) Run() error {
	listener, err := net.ListenMC(s.Address)
	if err != nil {
		return fmt.Errorf("failed to open server socket: %v", err)
	}

	log.Printf("Waiting for connections on %s", s.Address)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("Accept error: %v", err)
		}
		go s.acceptConn(conn)
	}
}

func (s *Server) acceptConn(conn net.Conn) {
	defer conn.Close()

	ipString := conn.Socket.RemoteAddr().String()
	log.Printf("New connection from %s\n", ipString)

	defer func() {
		if err := recover(); err != nil {
			log.Printf("catching panic: %v", err)
		}
	}()

	// handshake
	protocol, intention, err := s.handshake(conn)
	if err != nil {
		log.Printf("Handshake error: %v", err)
		return
	}

	session := Session{
		Server:          s,
		ProtocolVersion: protocol,
	}

	switch intention {
	default: //unknown error
		log.Printf("Unknown handshake intention: %v", intention)
	case 1: //for status
		session.acceptListPing(conn)
	case 2: //for login
		session.handlePlaying(conn, protocol)
	}
}

func (s *Session) handlePlaying(conn net.Conn, protocol int32) {
	// login, get player info
	info, err := s.acceptLogin(conn)
	if err != nil {
		log.Print("Login failed")
		return
	}

	// Write LoginSuccess packet
	if err = s.loginSuccess(conn, info.Name, info.UUID); err != nil {
		log.Print("Login failed on success")
		return
	}

	if err := s.joinGame(conn); err != nil {
		log.Print("Login failed on joinGame")
		return
	}

	if err := s.playerPositionAndLookClientbound(conn); err != nil {
		log.Printf("Login failed on sending PlayerPositionAndLookClientbound: %v", err)
		return
	}

	log.Printf("%s joined the server\n", info.Name)

	// Just for block this goroutine. Keep the connection
	for {
		var p pk.Packet
		if err := conn.ReadPacket(&p); err != nil {
			log.Printf("ReadPacket error: %v", err)
			break
		}

		var chatPacketId int32
		if s.ProtocolVersion == 754 {
			chatPacketId = packetid.ChatServerbound
		} else if s.ProtocolVersion >= 107 && s.ProtocolVersion <= 316 {
			chatPacketId = 0x02
		} else if s.ProtocolVersion == 335 {
			chatPacketId = 0x03
		} else if s.ProtocolVersion >= 338 && s.ProtocolVersion <= 404 {
			chatPacketId = 0x02
		} else if s.ProtocolVersion >= 477 {
			chatPacketId = 0x03
		} else {
			chatPacketId = 0x01
		}

		if p.ID == chatPacketId {
			var message pk.String
			if err := p.Scan(&message); err != nil {
				continue
			}

			if s.Server.ChatMessageCallback != nil {
				s.Server.ChatMessageCallback(string(message))
			}
		}
		// KeepAlive packet is not handled, so client might
		// exit because of "time out".
	}
}

type PlayerInfo struct {
	Name    string
	UUID    uuid.UUID
	OPLevel int
}

// acceptLogin check player's account
func (s *Session) acceptLogin(conn net.Conn) (info PlayerInfo, err error) {
	//login start
	var p pk.Packet
	err = conn.ReadPacket(&p)
	if err != nil {
		return
	}

	err = p.Scan((*pk.String)(&info.Name)) //decode username as pk.String
	if err != nil {
		return
	}

	info.UUID = offline.NameToUUID(info.Name)

	if s.Server.AcceptLoginCallback != nil {
		s.Server.AcceptLoginCallback(info.Name)
	}
	return
}

// handshake receive and parse Handshake packet
func (s *Server) handshake(conn net.Conn) (protocol, intention int32, err error) {
	var (
		p                   pk.Packet
		Protocol, Intention pk.VarInt
		ServerAddress       pk.String        // ignored
		ServerPort          pk.UnsignedShort // ignored
	)
	// receive handshake packet
	if err = conn.ReadPacket(&p); err != nil {
		return
	}
	err = p.Scan(&Protocol, &ServerAddress, &ServerPort, &Intention)

	log.Printf("Received handshake: %d %d %s:%d\n", Protocol, Intention, ServerAddress, ServerPort)

	return int32(Protocol), int32(Intention), err
}

// loginSuccess send LoginSuccess packet to client
func (s *Session) loginSuccess(conn net.Conn, name string, uuid uuid.UUID) error {
	if s.ProtocolVersion <= 4 {
		return conn.WritePacket(pk.Marshal(0x02,
			pk.String(strings.ReplaceAll(uuid.String(), "-", "")),
			pk.String(name),
		))
	} else if s.ProtocolVersion <= 578 {
		return conn.WritePacket(pk.Marshal(0x02,
			pk.String(uuid.String()),
			pk.String(name),
		))
	} else {
		return conn.WritePacket(pk.Marshal(0x02,
			pk.UUID(uuid),
			pk.String(name),
		))
	}
}

//go:embed DimensionCodec.snbt
var dimensionCodecSNBT string

//go:embed DimensionCodec2.snbt
var dimensionCodec2SNBT string

//go:embed Dimension.snbt
var dimensionSNBT string

func (s *Session) joinGame(conn net.Conn) error {
	if s.ProtocolVersion <= 47 && s.ProtocolVersion > 5 {
		return conn.WritePacket(pk.Marshal(0x01,
			pk.Int(0),          // EntityID
			pk.UnsignedByte(1), // Gamemode
			pk.Byte(0),
			pk.UnsignedByte(0),
			pk.UnsignedByte(MaxPlayer),
			pk.String("default"),
			pk.Boolean(true),
		))
	} else if s.ProtocolVersion == 107 {
		return conn.WritePacket(pk.Marshal(0x23,
			pk.Int(0),          // EntityID
			pk.UnsignedByte(1), // Gamemode
			pk.Byte(0),
			pk.UnsignedByte(0),
			pk.UnsignedByte(MaxPlayer),
			pk.String("default"),
			pk.Boolean(true),
		))
	} else if s.ProtocolVersion >= 108 && s.ProtocolVersion <= 340 {
		return conn.WritePacket(pk.Marshal(0x23,
			pk.Int(0),          // EntityID
			pk.UnsignedByte(1), // Gamemode
			pk.Int(0), // changed
			pk.UnsignedByte(0),
			pk.UnsignedByte(MaxPlayer),
			pk.String("default"),
			pk.Boolean(true),
		))
	} else if s.ProtocolVersion >= 393 && s.ProtocolVersion <= 404 {
		return conn.WritePacket(pk.Marshal(0x25,
			pk.Int(0),          // EntityID
			pk.UnsignedByte(1), // Gamemode
			pk.Int(0), // changed
			pk.UnsignedByte(0),
			pk.UnsignedByte(MaxPlayer),
			pk.String("default"),
			pk.Boolean(true),
		))
	} else if s.ProtocolVersion >= 477 && s.ProtocolVersion <= 498 {
		return conn.WritePacket(pk.Marshal(0x25,
			pk.Int(0),          // EntityID
			pk.UnsignedByte(1), // Gamemode
			pk.Int(0),
			pk.UnsignedByte(MaxPlayer),
			pk.String("default"),
			pk.VarInt(15),
			pk.Boolean(true),
		))
	} else if s.ProtocolVersion >= 573 && s.ProtocolVersion <= 578 {
		return conn.WritePacket(pk.Marshal(0x26,
			pk.Int(0),          // EntityID
			pk.UnsignedByte(1), // Gamemode
			pk.Int(0),
			pk.Long(0),
			pk.UnsignedByte(MaxPlayer),
			pk.String("default"),
			pk.VarInt(15),
			pk.Boolean(true),
			pk.Boolean(true),
		))
	} else if s.ProtocolVersion >= 735 && s.ProtocolVersion <= 736 {
		return conn.WritePacket(pk.Marshal(0x25,
			pk.Int(0),                                          // EntityID
			pk.UnsignedByte(1),                                 // Gamemode
			pk.UnsignedByte(1),                                 // Previous Gamemode
			pk.VarInt(1),                                       // World Count
			pk.Ary{Len: 1, Ary: []pk.Identifier{"world"}},      // World Names
			pk.NBT(nbt.StringifiedMessage(dimensionCodec2SNBT)), // Dimension codec
			pk.Identifier("overworld"),
			pk.Identifier("world"),                             // World Name
			pk.Long(0),                                         // Hashed Seed
			pk.VarInt(MaxPlayer),                               // Max Players
			pk.VarInt(15),                                      // View Distance
			pk.Boolean(false),                                  // Reduced Debug Info
			pk.Boolean(true),                                   // Enable respawn screen
			pk.Boolean(false),                                  // Is Debug
			pk.Boolean(true),                                   // Is Flat
		))
	} else if s.ProtocolVersion >= 751 {
		return conn.WritePacket(pk.Marshal(0x24,
			pk.Int(0),                                          // EntityID
			pk.Boolean(false),                                  // Is hardcore
			pk.UnsignedByte(1),                                 // Gamemode
			pk.Byte(1),                                         // Previous Gamemode
			pk.VarInt(1),                                       // World Count
			pk.Ary{Len: 1, Ary: []pk.Identifier{"world"}},      // World Names
			pk.NBT(nbt.StringifiedMessage(dimensionCodecSNBT)), // Dimension codec
			pk.NBT(nbt.StringifiedMessage(dimensionSNBT)),      // Dimension
			pk.Identifier("world"),                             // World Name
			pk.Long(0),                                         // Hashed Seed
			pk.VarInt(MaxPlayer),                               // Max Players
			pk.VarInt(15),                                      // View Distance
			pk.Boolean(false),                                  // Reduced Debug Info
			pk.Boolean(true),                                   // Enable respawn screen
			pk.Boolean(false),                                  // Is Debug
			pk.Boolean(true),                                   // Is Flat
		))
	} else {
		return conn.WritePacket(pk.Marshal(0x01,
			pk.Int(0),          // EntityID
			pk.UnsignedByte(1), // Gamemode
			pk.Byte(0),
			pk.UnsignedByte(0),
			pk.UnsignedByte(MaxPlayer),
			pk.String("default"),
		))
	}
}

func (s *Session) playerPositionAndLookClientbound(conn net.Conn) error {
	if s.ProtocolVersion == 754 {
		return conn.WritePacket(pk.Marshal(0x34,
			pk.Double(0), pk.Double(0), pk.Double(0), // XYZ
			pk.Float(0), pk.Float(0), // Yaw Pitch
			pk.Byte(0),   // flag
			pk.VarInt(0), // TP ID
		))
	} else if s.ProtocolVersion >= 107 && s.ProtocolVersion <= 335 {
		return conn.WritePacket(pk.Marshal(0x2e,
			pk.Double(0), pk.Double(0), pk.Double(0), // XYZ
			pk.Float(0), pk.Float(0), // Yaw Pitch
			pk.Byte(0),   // flag
			pk.VarInt(0), // TP ID
		))
	} else if s.ProtocolVersion >= 338 && s.ProtocolVersion <= 340 {
		return conn.WritePacket(pk.Marshal(0x2f,
			pk.Double(0), pk.Double(0), pk.Double(0), // XYZ
			pk.Float(0), pk.Float(0), // Yaw Pitch
			pk.Byte(0),   // flag
			pk.VarInt(0), // TP ID
		))
	} else if s.ProtocolVersion >= 393 && s.ProtocolVersion <= 404 {
		return conn.WritePacket(pk.Marshal(0x32,
			pk.Double(0), pk.Double(0), pk.Double(0), // XYZ
			pk.Float(0), pk.Float(0), // Yaw Pitch
			pk.Byte(0),   // flag
			pk.VarInt(0), // TP ID
		))
	} else if s.ProtocolVersion >= 477 && s.ProtocolVersion <= 498 {
		return conn.WritePacket(pk.Marshal(0x35,
			pk.Double(0), pk.Double(0), pk.Double(0), // XYZ
			pk.Float(0), pk.Float(0), // Yaw Pitch
			pk.Byte(0),   // flag
			pk.VarInt(0), // TP ID
		))
	} else if s.ProtocolVersion >= 573 && s.ProtocolVersion <= 578 {
		return conn.WritePacket(pk.Marshal(0x36,
			pk.Double(0), pk.Double(0), pk.Double(0), // XYZ
			pk.Float(0), pk.Float(0), // Yaw Pitch
			pk.Byte(0),   // flag
			pk.VarInt(0), // TP ID
		))
	} else if s.ProtocolVersion == 735 {
		return conn.WritePacket(pk.Marshal(0x35,
			pk.Double(0), pk.Double(0), pk.Double(0), // XYZ
			pk.Float(0), pk.Float(0), // Yaw Pitch
			pk.Byte(0),   // flag
			pk.VarInt(0), // TP ID
		))
	} else if s.ProtocolVersion >= 736 {
		return conn.WritePacket(pk.Marshal(0x34,
			pk.Double(0), pk.Double(0), pk.Double(0), // XYZ
			pk.Float(0), pk.Float(0), // Yaw Pitch
			pk.Byte(0),   // flag
			pk.VarInt(0), // TP ID
		))
	} else {
		return conn.WritePacket(pk.Marshal(0x08,
			pk.Double(0), pk.Double(0), pk.Double(0), // XYZ
			pk.Float(0), pk.Float(0), // Yaw Pitch
			pk.Boolean(false), // flag
		))
	}
}
