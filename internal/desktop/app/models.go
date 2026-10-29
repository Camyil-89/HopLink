package desktop

import (
	"crypto/rand"
	"encoding/hex"
)

const (
	defaultPort = 22033
	serviceName = "hoplink.service"
	remoteDir   = "/opt/hoplink"
)

type Server struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Host             string  `json:"host"`
	SSHPort          int     `json:"sshPort"`
	SSHUser          string  `json:"sshUser"`
	SSHPassword      string  `json:"sshPassword"`
	SSHPrivateKey    string  `json:"sshPrivateKey,omitempty"`
	SSHKeyPassphrase string  `json:"sshKeyPassphrase,omitempty"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	Role             string  `json:"role,omitempty"`
}
type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Link     string `json:"link"`
	Active   *bool  `json:"active,omitempty"`
}

func (u User) isActive() bool { return u.Active == nil || *u.Active }

func boolPointer(value bool) *bool { return &value }

type Edge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}
type AppConfig struct {
	InstallSource   string            `json:"installSource,omitempty"`
	Servers         []Server          `json:"servers"`
	MapServers      []Server          `json:"mapServers"`
	DeployedServers []Server          `json:"deployedServers,omitempty"`
	Users           []User            `json:"users"`
	Edges           []Edge            `json:"edges"`
	Routes          []Route           `json:"routes"`
	ActiveRouteID   string            `json:"activeRouteId"`
	EntryDomain     string            `json:"entryDomain"`
	SNI             string            `json:"sni"`
	Port            int               `json:"port"`
	ChainKey        string            `json:"chainKey"`
	Certificate     string            `json:"certificate"`
	CertificateKey  string            `json:"certificateKey"`
	LogLevel        string            `json:"logLevel,omitempty"`
	KnownHosts      map[string]string `json:"knownHosts"`
}
type Route struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Index           int      `json:"index"`
	LegacyLayout    bool     `json:"legacyLayout,omitempty"`
	LegacyIDLayout  bool     `json:"legacyIdLayout,omitempty"`
	EntryDomain     string   `json:"entryDomain,omitempty"`
	MapServers      []Server `json:"mapServers"`
	DeployedServers []Server `json:"deployedServers,omitempty"`
	Users           []User   `json:"users"`
	Edges           []Edge   `json:"edges"`
	SNI             string   `json:"sni"`
	Port            int      `json:"port"`
	ChainKey        string   `json:"chainKey"`
	Certificate     string   `json:"certificate"`
	CertificateKey  string   `json:"certificateKey"`
	LogLevel        string   `json:"logLevel,omitempty"`
}
type MapState struct {
	Servers []Server `json:"servers"`
	Edges   []Edge   `json:"edges"`
}

func newID() string { b := make([]byte, 10); _, _ = rand.Read(b); return hex.EncodeToString(b) }
