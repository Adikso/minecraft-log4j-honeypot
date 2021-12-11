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
)

const (
	PlayerPositionAndLookClientbound = 0x34
	JoinGame                         = 0x24
	ProtocolVersion                  = 754
	MaxPlayer                        = 50
)

type Server struct {
	Address string
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

	switch intention {
	default: //unknown error
		log.Printf("Unknown handshake intention: %v", intention)
	case 1: //for status
		s.acceptListPing(conn)
	case 2: //for login
		s.handlePlaying(conn, protocol)
	}
}

func (s *Server) handlePlaying(conn net.Conn, protocol int32) {
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
	if err := conn.WritePacket(pk.Marshal(PlayerPositionAndLookClientbound,
		pk.Double(0), pk.Double(0), pk.Double(0), // XYZ
		pk.Float(0), pk.Float(0), // Yaw Pitch
		pk.Byte(0),        // flag
		pk.VarInt(0),      // TP ID
	)); err != nil {
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

		if p.ID == packetid.ChatServerbound {
			var message pk.String
			if err := p.Scan(&message); err != nil {
				continue
			}

			if s.ChatMessageCallback != nil {
				s.ChatMessageCallback(string(message))
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
func (s *Server) acceptLogin(conn net.Conn) (info PlayerInfo, err error) {
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

	if s.AcceptLoginCallback != nil {
		s.AcceptLoginCallback(info.Name)
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
func (s *Server) loginSuccess(conn net.Conn, name string, uuid uuid.UUID) error {
	return conn.WritePacket(pk.Marshal(0x02,
		pk.UUID(uuid),
		pk.String(name),
	))
}

//go:embed DimensionCodec.snbt
var dimensionCodecSNBT string

//go:embed Dimension.snbt
var dimensionSNBT string

func (s *Server) joinGame(conn net.Conn) error {
	return conn.WritePacket(pk.Marshal(JoinGame,
		pk.Int(0),          // EntityID
		pk.Boolean(false),  // Is hardcore
		pk.UnsignedByte(1), // Gamemode
		pk.Byte(1),         // Previous Gamemode
		pk.VarInt(1),       // World Count
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
}
