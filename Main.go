package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os/exec"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/mediadevices"

	"github.com/pion/mediadevices/pkg/codec/vpx"
	_ "github.com/pion/mediadevices/pkg/driver/camera" // uncomment this for actual camera

	// _ "github.com/pion/mediadevices/pkg/driver/videotest" //comment this for actual camera
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/webrtc/v3"
)

var (
	api         *webrtc.API
	botVideo    mediadevices.MediaStream //this is constant after running VideoSetup(), therefore it is thread safe
	pythonInput io.WriteCloser
	wsUpgrader  = websocket.Upgrader{
		ReadBufferSize:  512,
		WriteBufferSize: 512,
	}
	hasVideo bool
)

func main() {
	hasVideo = VideoSetup()
	PythonGPIOSetup()
	http.Handle(`/`, http.FileServer(http.Dir(`frontend`))) //serve frontend
	http.HandleFunc(`/signaler`, SignalingServer)           //handle websocket requests
	interfaces, e := net.InterfaceAddrs()
	if e != nil {
		panic(fmt.Errorf(`failed to read all network interfaces. reason: %v`, e))
	}
	for _, address := range interfaces {
		if ip, ok := address.(*net.IPNet); ok && !ip.IP.IsLoopback() {
			if ip.IP.To4() != nil {
				log.Println(`Server Initialized on: https://`+ip.IP.String(), `Please enter this in the browser to view the video feed.`)
				break
			}
		}
	}
	log.Println(`Server Initialized`)
	panic(http.ListenAndServeTLS(`:443`, `server.crt`, `server.key`, nil)) //start server
}

type SignalingSocket struct { //thread safe websocket
	*websocket.Conn
	*sync.Mutex
}

type Signal struct { //JSON format for signals
	Event string `json:"event"`
	Data  string `json:"data"`
}

func (ws *SignalingSocket) SendSignal(s Signal) {
	ws.Lock()
	defer ws.Unlock()
	ws.WriteJSON(s) //this will never fail so the error is ignored
}

func VideoSetup() bool {
	//videoCodecParams, e := mmal.NewParams() //h264 video codec but optimized for the pi (videocore gpu)
	videoCodecParams, e := vpx.NewVP8Params() //vp8 codec is the default webrtc codec, if mobile doesnt like mmal then swap to this (there will be a performance hit tho)
	//videoCodecParams, e := openh264.NewParams() //openh264 for windows debugging
	if e != nil {
		log.Println(fmt.Errorf(`Failed to get codec parameters %v`, e))
		return false
	}
	videoCodecParams.KeyFrameInterval = 60
	videoCodecParams.BitRate = 1_000_000 //1Mbps bitrate
	mediaEngine := webrtc.MediaEngine{}
	codecSelector := mediadevices.NewCodecSelector(mediadevices.WithVideoEncoders(&videoCodecParams))
	codecSelector.Populate(&mediaEngine)
	api = webrtc.NewAPI(webrtc.WithMediaEngine(&mediaEngine)) //create webrtc api instance that supports our video
	botVideo, e = mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		Video: func(constraint *mediadevices.MediaTrackConstraints) {
			constraint.FrameFormat = prop.FrameFormat(frame.FormatYUY2) //YUY2 compression format
			constraint.Width = prop.Int(640)                            //480p
			constraint.Height = prop.Int(480)
		},
		Codec: codecSelector,
	})
	if e != nil {
		log.Println(fmt.Errorf(`Failed to find camera with valid parameters: %v`, e))
		return false
	}
	log.Println(`Successfully initialized camera`)
	return true
}

func PythonGPIOSetup() {
	var e error
	py := exec.Command(`python`, `controls.py`) //'py' for windows, 'python' for unix
	pythonInput, e = py.StdinPipe()
	if e != nil {
		panic(fmt.Errorf(`Failed to pipe STDIN for 'controls.py': %v`, e))
	}
	stderrPipe, e := py.StderrPipe()
	if e != nil {
		panic(fmt.Errorf(`Failed to pipe STDERR for 'controls.py': %v`, e))
	}
	// go func() {
	// 	scan := bufio.NewScanner(stderrPipe)
	// 	scan.Split(bufio.ScanBytes)
	// 	for scan.Scan() {
	// 		fmt.Print(scan.Text())
	// 	}
	// }()
	// stdoutPipe, e := py.StdoutPipe() // if u want to be able to read STDOUT from python, this is how
	// if e != nil {
	// 	panic(fmt.Errorf(`failed to pipe STDOUT for 'controls.py': %v`, e))
	// }
	if e := py.Start(); e != nil {
		panic(fmt.Errorf(`Failed to start 'controls.py': %v`, e))
	}
	log.Println(`Successfully initialized Python controls script`)
}

func SignalingServer(w http.ResponseWriter, r *http.Request) {
	ws, e := wsUpgrader.Upgrade(w, r, nil) //upgrade http request into a websocket
	if e != nil {
		http.Error(w, e.Error(), http.StatusInternalServerError)
		return
	}
	signaler := &SignalingSocket{ws, &sync.Mutex{}} //create a thread safe websocket
	defer signaler.Close()

	peer, e := api.NewPeerConnection(webrtc.Configuration{}) //create a webrtc instance, i dont specify ICE servers bc this is on LAN
	if e != nil {
		panic(fmt.Errorf(`Failed to create WebRTC peer connection: %v`, e))
	}
	defer peer.Close()

	peer.OnICECandidate(func(ice *webrtc.ICECandidate) {
		if ice != nil { //send out ICE candidates once we generate them
			bin, _ := json.Marshal(ice.ToJSON())
			signaler.SendSignal(Signal{Event: `ice`, Data: string(bin)})
		}
	})

	peer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateFailed {
			panic(`WebRTC disconnected and/or failed`)
		}
	})

	if hasVideo {
		for _, track := range botVideo.GetVideoTracks() { //add video track to webrtc peer instance
			if _, e := peer.AddTransceiverFromTrack(track,
				webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly}); e != nil {
				panic(fmt.Errorf(`Failed to add video track to server peer: %v`, e))
			}
		}
	}
	notTrue := false
	controlChan, e := peer.CreateDataChannel(`controls`, &webrtc.DataChannelInit{Ordered: &notTrue})
	if e != nil {
		panic(fmt.Errorf(`Failed to create control data channel for client from server: %v`, e))
	}
	controlChan.OnMessage(func(msg webrtc.DataChannelMessage) {
		if msg.IsString { //whenever we receive a message from webrtc, pass it to python
			if _, e := pythonInput.Write(append(msg.Data, '\n')); e != nil {
				log.Fatal(fmt.Errorf(`Failed to pass input from WebRTC to 'controls.py': %v`, e)) //fatal, if python doesnt work then the whole system is gone
			}
		}
	})

	offer, e := peer.CreateOffer(nil) //send out offer to peer to watch video and send control signals
	if e != nil {
		panic(fmt.Errorf(`Failed to create offer for the server: %v`, e))
	}

	if e := peer.SetLocalDescription(offer); e != nil {
		panic(fmt.Errorf(`Failed to set local description for server peer: %v`, e))
	}

	bin, _ := json.Marshal(offer)
	signaler.SendSignal(Signal{Event: `offer`, Data: string(bin)}) //send the initial offer
	signal := Signal{}
	for {
		if e := signaler.ReadJSON(&signal); e != nil {
			panic(fmt.Errorf(`Failed to parse JSON from server: %v`, e))
		}
		switch signal.Event {
		case `ice`:
			ice := webrtc.ICECandidateInit{}
			if e := json.Unmarshal([]byte(signal.Data), &ice); e != nil {
				panic(fmt.Errorf(`Failed to parse JSON ICE candidate from server: %v`, e))
			}
			if e := peer.AddICECandidate(ice); e != nil {
				panic(fmt.Errorf(`Failed to add ICE candidate from server: %v`, e))
			}
			if peer.ConnectionState() == webrtc.PeerConnectionStateConnected {
				log.Println(`Successfully streaming camera feed to client`)
			}
		case `answer`:
			answer := webrtc.SessionDescription{}
			if e := json.Unmarshal([]byte(signal.Data), &answer); e != nil {
				panic(fmt.Errorf(`Failed to parse JSON answer from server: %v`, e))
			}
			if e := peer.SetRemoteDescription(answer); e != nil {
				panic(fmt.Errorf(`Failed to set remote description for server peer: %v`, e))
			}
		}
	}
}

func KillMotors() {
	/*
			fd = open("/sys/class/gpio/gpio24/direction", O_WRONLY);
		    if (fd == -1) {
		        perror("Unable to open /sys/class/gpio/gpio24/direction");
		        exit(1);
		    }

		    if (write(fd, "out", 3) != 3) {
		        perror("Error writing to /sys/class/gpio/gpio24/direction");
		        exit(1);
		    }
			relay1_pin = 11
			relay2_pin = 13

			m1_dir_pin = 16
			m2_dir_pin = 31

			m1_pwm_pin = 12
			m2_pwm_pin = 33
	*/
	// var (
	// 	relay1Pin = 11
	// 	relay2Pin = 13
	// 	m1DirPin  = 16
	// 	m2DirPin  = 31
	// 	m1PwmPin  = 12
	// 	m2PwmPin  = 33
	// )
	// for _, v := range []int{relay1Pin, relay2Pin, m1DirPin, m2DirPin, m1PwmPin, m2PwmPin} {
	// 	os.WriteFile(fmt.Sprintf(`/sys/class/gpio/gpio%v/value`, v), []byte(`0`), fs.FileMode(os.O_WRONLY))
	// }
}
