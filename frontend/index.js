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
        document.body.appendChild(webcam);
        streams[0].onended = () => alert(`Disconnected`);
    };
    rtc.ondatachannel = ({channel}) => { //when we receive a data channel
        if(channel.label != `controls`) return;
        console.log(`got controls channel!`);
        const inputForm = document.getElementById(`request-sender`);
        inputForm.onsubmit = () => { //configure the `request-sender` form to pass text to python
            channel.send(inputForm.children[0].value);
            inputForm.children[0].value = ``; //clear output
            return false;
        };
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
                console.log(`got ice!`, content);
                rtc.addIceCandidate(content); //add ice candidates
                break;
            default:
                console.log(`Invalid message:`, content);
        }
    };
};
ws.onclose = ws.onerror = ({reason}) => alert(`Disconnected ${reason}`);