package config

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	//"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/transport"
)

var configFile = flag.String("config", "config.json", "Configuration file for bamboo replica. Defaults to config.json.")

// Config contains every system configuration
type Config struct {
	Addrs     map[identity.NodeID]string `json:"address"`      // address for mempool communication
	Addrs2    map[identity.NodeID]string `json:"address2"`     // address for consensus communication
	HTTPAddrs map[identity.NodeID]string `json:"http_address"` // address for client server communication

	Policy    string  `json:"policy"`    // leader change policy {consecutive, majority}
	Threshold float64 `json:"threshold"` // threshold for policy in WPaxos {n consecutive or time interval in ms}

	Thrifty          bool            `json:"thrifty"`          // only send messages to a quorum
	BufferSize       int             `json:"buffer_size"`      // buffer size for maps
	ChanBufferSize   int             `json:"chan_buffer_size"` // buffer size for channels
	MultiVersion     bool            `json:"multiversion"`     // create multi-version database
	Timeout          int             `json:"timeout"`
	ByzNo            int             `json:"byzNo"`
	BSize            int             `json:"bsize"` // max number of microblock contained in a block
	MSize            int             `json:"msize"` // byte size of a microblock
	Fixed            bool            `json:"fixed"`
	Benchmark        Bconfig         `json:"benchmark"` // benchmark configuration
	Delta            int             `json:"delta"`     // timeout, seconds
	Pprof            bool            `json:"pprof"`
	MaxRound         int             `json:"maxRound"`
	Strategy         string          `json:"strategy"`
	ProposeTime      int             `json:"propose_time"`
	PayloadSize      int             `json:"payload_size"`
	Master           identity.NodeID `json:"master"`
	Delay            int             `json:"delay"`   // transmission delay in ms
	DErr             int             `json:"derr"`    // the err taken into delays
	MemSize          int             `json:"memsize"` // max number of microblocks in the mempool
	Slow             int             `json:"slow"`
	Crash            int             `json:"crash"`
	MemType          string          `json:"mem_type"`
	EstimateNum      int             `json:"estimate_num"`
	EstimateWindow   int             `json:"estimate_window"`
	DefaultDelay     int             `json:"default_delay"`
	Gossip           bool            `json:"gossip"`
	Fanout           int             `json:"fanout"`
	SlowFanout       int             `json:"slow_fanout"`
	FillInterval     int             `json:"fillInterval"`
	Capacity         int             `json:"capacity"`
	P                int             `json:"p"`
	SlowNo           int             `json:"slow_no"`
	Q                int             `json:"q"` // the number of acks to be considered stable
	R                int             `json:"r"` // max hops of gossip
	Zipf             bool            `json:"zipf"`
	Opt              bool            `json:"opt"`
	LoadBalance      bool            `json:"load_balance"`
	LoadedIndex      int             `json:"loaded_index"`
	ForwardP         int             `json:"forward_p"`
	D                int             `json:"d"`
	BroadcastByGroup bool            `json:"broadcastBygroup"`
	Onlyworker       bool            `json:"onlyworker"`

	// zipfian distribution
	ZipfianS float64 `json:"zipfian_s"` // zipfian s parameter
	ZipfianV float64 `json:"zipfian_v"` // zipfian v parameter

	hasher string
	signer string

	// for future implementation
	// Batching bool `json:"batching"`
	// Consistency string `json:"consistency"`
	// Codec string `json:"codec"` // codec for message serialization between nodes

	n int // total number of nodes
	//z   int         // total number of zones
	//npz map[int]int // nodes per zone

	//分组

	GroupNum  int `json:"groupNum"`  //有多少分组
	MemberNum int `json:"memberNum"` //分组中成员的数量 2f+1
	Time      int `json:"time"`      //分组中成员的数量 2f+1

	//交易池
	Poolsize    int    `json:"poolsize"` //交易池大小
	Model       string `json:"model"`    //交易到来模式仿真
	TxPerSecond int    `json:"txPerSecond"`

	//限制微块广播频率
	Mb_broadcast int `json:"mb_broadcast"`
}

//var keys []crypto.PrivateKey
//var pubKeys []crypto.PublicKey

// Bconfig holds all benchmark configuration
type Bconfig struct {
	T            int    // total number of running time in seconds
	N            int    // total number of requests
	K            int    // key sapce
	Throttle     int    // requests per second throttle, unused if 0
	Concurrency  int    // number of simulated clients
	Distribution string // distribution
	// rounds       int    // repeat in many rounds sequentially
	Cold bool //是否冷启动更多
	// conflict distribution
	Conflicts int // percentage of conflicting keys
	Min       int // min key

	// normal distribution
	Mu    float64 // mu of normal distribution
	Sigma float64 // sigma of normal distribution
	Move  bool    // moving average (mu) of normal distribution
	Speed int     // moving speed in milliseconds intervals per key
}

// Config is global configuration singleton generated by init() func below
var Configuration Config

func init() {
	Configuration = MakeDefaultConfig()
}

// GetConfig returns paxi package configuration
func GetConfig() Config {
	return Configuration
}

func GetTimer() time.Duration {
	return time.Duration(time.Duration(Configuration.Timeout) * time.Millisecond)
}

// Simulation enable go channel transportation to simulate distributed environment
func Simulation() {
	*transport.Scheme = "chan"
}

// MakeDefaultConfig returns Config object with few default values
// only used by init() and master
func MakeDefaultConfig() Config {
	return Config{
		Policy:         "consecutive",
		Threshold:      3,
		BufferSize:     1024,
		ChanBufferSize: 1024,
		MultiVersion:   false,
		hasher:         "sha3_256",
		signer:         "ECDSA_P256",
		//Benchmark:      DefaultBConfig(),
	}
}

//func SetKeys() error {
//	keys = make([]crypto.PrivateKey, Configuration.N())
//	pubKeys = make([]crypto.PublicKey, Configuration.N())
//	var err error
//	for i := 0; i < Configuration.N(); i++ {
//		keys[i], err = crypto.GenerateKey(Configuration.signer)
//		if err != nil {
//			return err
//		}
//		pubKeys[i] = keys[i].PublicKey()
//	}
//	return nil
//}

//func (c Config) GetKeys(id int) (*crypto.PrivateKey, []crypto.PublicKey) {
//	return &keys[id], pubKeys
//}

// IDs returns all node ids
func (c Config) IDs() []identity.NodeID {
	ids := make([]identity.NodeID, 0)
	for id := range c.Addrs {
		ids = append(ids, id)
	}
	return ids
}

// N returns total number of nodes
func (c Config) N() int {
	return c.n
}

// GetHash returns the hashing scheme of the configuration
func (c Config) GetHashScheme() string {
	return c.hasher
}

func (c Config) GetSignatureScheme() string {
	return c.signer
}

// GetSignatureScheme returns the signing scheme of the configuration

// Z returns total number of zones
//func (c Config) Z() int {
//	return c.z
//}

// String is implemented to print the Configuration
func (c Config) String() string {
	config, err := json.Marshal(c)
	if err != nil {
		log.Error(err)
		return ""
	}
	return string(config)
}

// Load loads configuration from Configuration file in JSON format
func (c *Config) Load() {
	file, err := os.Open(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(c)
	if err != nil {
		log.Fatal(err)
	}

	// load ips
	ip_file, err := os.Open("ips.txt")
	if err != nil {
		fmt.Println(err)
	}
	defer ip_file.Close()

	scanner := bufio.NewScanner(ip_file)
	i := 1
	for scanner.Scan() {
		id := identity.NewNodeID(i)
		port := strconv.Itoa(3734 + i)
		port2 := strconv.Itoa(2628 + i)
		addr := "tcp://" + scanner.Text() + ":" + port
		addr2 := "tcp://" + scanner.Text() + ":" + port2
		portHttp := strconv.Itoa(8069 + i)
		addrHttp := "http://" + scanner.Text() + ":" + portHttp
		c.Addrs[id] = addr
		c.Addrs2[id] = addr2
		c.HTTPAddrs[id] = addrHttp
		i++
	}

	if err := scanner.Err(); err != nil {
		fmt.Println(err)
	}

	c.n = len(c.Addrs)
}

// Save saves configuration to file in JSON format
func (c Config) Save() error {
	file, err := os.Create(*configFile)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	return encoder.Encode(c)
}

func (c Config) IsByzantine(id identity.NodeID) bool {
	return c.ByzNo >= id.Node()
}
