var enable = false, speed = 10;

function formatSignal(event, data) {
    return JSON.stringify({
        event: event,
        data: JSON.stringify(data)
    });
}

const ws = new WebSocket(`wss://${location.hostname}:${location.port}/signaler`);
ws.onopen = () => {
    console.log(`successfully connected to the backend!`);
    const rtc = new RTCPeerConnection({iceServers: [{urls: `stun:stun.l.google.com:19302`}]}); //create a WebRTC instance
    rtc.onicecandidate = ({candidate}) => candidate && ws.send(formatSignal(`ice`, candidate)); //if the ice candidate is not null, send it to the peer
    rtc.oniceconnectionstatechange = () => rtc.iceConnectionState == `failed` && rtc.restartIce();
    rtc.ontrack = ({streams}) => { //get remote feed and throw into a <video>
        const webcam = document.createElement(`video`);
        webcam.controls = webcam.autoplay = true;
        webcam.srcObject = streams[0];
        const vidSection = document.getElementById("video");
        vidSection.appendChild(webcam);
        streams[0].onended = () => alert(`Disconnected`);
    };
    rtc.ondatachannel = ({channel}) => { //when we receive a data channel
        if(channel.label != `controls`) return;
        console.log(`got controls channel!`);
        const controls = document.getElementById(`controls`);
        controls.addEventListener("movement", function(e){
            console.log("" + e.detail.en + "::" + e.detail.motor_1 + "::" + e.detail.motor_2 + "::" + e.detail.s);
            channel.send("" + e.detail.en + "::" + e.detail.motor_1 + "::" + e.detail.motor_2 + "::" + e.detail.s);
            return false;
        });

        window.addEventListener("keydown", function(e){
            keyPress(e, channel);
        });
        window.addEventListener(`keyup`, e => {
            channel.send("" + enable + "::" + 0 + "::" + 0 + "::" + speed); 
        });
    };
    ws.onmessage = async ({data}) => { //signal handler
        const signal = JSON.parse(data),
              content = JSON.parse(signal.data);
        switch(signal.event) {
            case `offer`:
                console.log(`got offer!`, content);
                await rtc.setRemoteDescription(content); //accept offer
                const answer = await rtc.createAnswer();
                await rtc.setLocalDescription(answer);
                ws.send(formatSignal(`answer`, answer)); //send answer
                console.log(`sent answer!`, answer);
                break;
            case `answer`:
                console.log(`got answer!`, content);
                await rtc.setRemoteDescription(content); //accept answer
                break;
            case `ice`:
                if(!content) return;
                console.log(`got ice!`, content);
                rtc.addIceCandidate(content); //add ice candidates
                break;
            default:
                console.log(`Invalid message:`, content);
        }
    };
};
ws.onclose = ws.onerror = ({reason}) => alert(`Disconnected ${reason}`);

function move(direction){
    let m1 = m2 = 0;
    switch(direction){
    case 'UL':
        m1 = 50; m2 = 100;
        break;
    case 'U':
        m1 = 100; m2 = 100;
        break;
    case 'UR':
        m1 = 100; m2 = 50;
        break;
    case 'L':
        m1 = -50; m2 = 50;
        break;
    case 'C':
        m1 = 0; m2 = 0;
        break;
    case 'R':
        m1 = 50; m2 = -50;
        break;
    case 'DL':
        m1 = -50; m2 = -100;
        break;
    case 'D':
        m1 = -100; m2 = -100;
        break;
    case 'DR':
        m1 = -100; m2 = -50;
        break;
    }

    var event = new CustomEvent('movement', {detail: {en:enable, motor_1:m1, motor_2:m2, s:speed}});
    document.getElementById("controls").dispatchEvent(event);
}

function keyPress(e, channel){
    var map = {};
    let m1 = m2 = 0;
    e = e || event;
    map[e.key] = e.type == 'keydown';

    console.log("is happening")

    if(map["ArrowUp"] && map["ArrowLeft"]){
        m1 = 50; m2 = 100;
    }else if(map["ArrowUp"] && map["ArrowRight"]){
        m1 = 100; m2 = 50;
    }else if(map["ArrowDown"] && map["ArrowLeft"]){
        m1 = -50; m2 = -100;
    }else if(map["ArrowDown"] && map["ArrowRight"]){
        m1 = -100; m2 = -50;
    }else if(map["ArrowUp"]){
        m1 = 100; m2 = 100;
    }else if(map["ArrowDown"]){
        m1 = -100; m2 = -100;
    }else if(map["ArrowLeft"]){
        m1 = -50; m2 = 50;
    }else if(map["ArrowRight"]){
        m1 = 50; m2 = -50;
    }else{
        m1 = 0; m2 = 0;
    }

    channel.send("" + enable + "::" + m1 + "::" + m2 + "::" + speed);
    console.log("" + enable + "::" + m1 + "::" + m2 + "::" + speed);
    // var event = new Event('movement', {en: enable, motor_1:m1, motor_2:m2, s:speed});
    // document.getElementById("controls").dispatchEvent(event);
}