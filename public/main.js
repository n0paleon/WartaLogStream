const params = new URLSearchParams(window.location.search);
const sessionId = params.get("session");
const errorMessageDiv = document.getElementById('error-message');
const statusIndicator = document.getElementById('status-indicator');
const statusText = statusIndicator.querySelector('.status-text');
const statusDot = statusIndicator.querySelector('.status-dot');
const statusLabel = document.getElementById('status-label');
const sessionAgeRow = document.getElementById('session-age-row');
const sessionAgeValue = document.getElementById('session-age-value');

let sessionCreationTime = null;
let sessionStopTime = null;
let ageInterval = null;

function formatDuration(seconds) {
    if (seconds < 0) seconds = 0;
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = seconds % 60;
    return [
        h.toString().padStart(2, '0'),
        m.toString().padStart(2, '0'),
        s.toString().padStart(2, '0')
    ].join(':');
}

function updateSessionAge() {
    if (sessionCreationTime && !sessionStopTime) {
        const now = Math.floor(Date.now() / 1000);
        sessionAgeValue.textContent = formatDuration(now - sessionCreationTime);
    }
}

function showSessionAgeRow() {
    sessionAgeRow.style.display = "flex";
}

function setStatus(status) {
    if (status === "Running") {
        statusText.textContent = "Running";
        statusIndicator.classList.remove("status-stopped");
        statusIndicator.classList.add("status-running");
        statusDot.classList.remove("status-stopped");
        statusDot.classList.add("status-running");
        // resume session age updates if creation time exists and session not stopped
        if (sessionCreationTime && !sessionStopTime && !ageInterval) {
            ageInterval = setInterval(updateSessionAge, 1000);
        }
    } else {
        statusText.textContent = "Stopped";
        statusIndicator.classList.remove("status-running");
        statusIndicator.classList.add("status-stopped");
        statusDot.classList.remove("status-running");
        statusDot.classList.add("status-stopped");
        // Stop session age updates
        if (ageInterval) {
            clearInterval(ageInterval);
            ageInterval = null;
        }
    }
}

// initial status
setStatus("Stopped");

if (!sessionId) {
    errorMessageDiv.innerHTML = `
        <div class="alert alert-warning text-center" role="alert" style="border-radius:12px;">
            <strong>Session ID Required</strong><br>
            Please provide a session ID in the URL, e.g. <code>?session=ULID12345</code>
        </div>
    `;
    errorMessageDiv.style.display = "block";
    throw new Error("Session ID missing");
}

const term = new Terminal({
    cursorBlink: true,
    cursorStyle: "bar",
    cursorWidth: 5,
    cursorInactiveStyle: "bar",
    theme: { background: "#222", foreground: "#e0e7ff", cursor: "#ffffffff" },
    fontFamily: "'JetBrains Mono', 'Fira Mono', monospace",
    scrollback: 200,
    disableStdin: true,
});

term.open(document.getElementById('terminal'));

term.focus();

const noteColumn = document.getElementById('note-column');

const ws = new WebSocket(`wss://${window.location.host}/session/${sessionId}/ws/subscriber`);

// event log / note / status / session_creation_time / session_stop_time
ws.addEventListener('message', (evt) => {
    const data = JSON.parse(evt.data);

    if (data.type === "session_creation_time") {
        sessionCreationTime = Number(data.data);
        sessionStopTime = null;
        showSessionAgeRow();
        updateSessionAge();
        if (ageInterval) clearInterval(ageInterval);
        ageInterval = setInterval(updateSessionAge, 1000);
        return;
    }

    if (data.type === "session_stop_time") {
        sessionStopTime = Number(data.data);
        if (ageInterval) {
            clearInterval(ageInterval);
            ageInterval = null;
        }
        if (sessionCreationTime) {
            sessionAgeValue.textContent = formatDuration(sessionStopTime - sessionCreationTime);
        }
        return;
    }

    if (data.type === "status") {
        setStatus(data.data);
        if (data.data === "Stopped") {
            ws.close();
        }
        return;
    }

    if (data.message === "invalid session id") {
        errorMessageDiv.innerHTML = `
            <div class="alert alert-danger text-center" role="alert" style="border-radius:12px;">
                <strong>Invalid Session ID</strong><br>
                The session ID provided is not valid. Please check your URL and try again.
            </div>
        `;
        errorMessageDiv.style.display = "block";
        document.getElementById('main-container').classList.add('blur-bg');
        ws.close();
        setStatus("Stopped");
        if (ageInterval) clearInterval(ageInterval);
        sessionAgeRow.style.display = "none";
        return;
    }

    if(data.type === "log") {
        term.writeln(data.data);
    } else if(data.type === "note") {
        const noteDiv = document.createElement('div');
        noteDiv.className = 'note';
        noteDiv.textContent = data.data;
        noteColumn.appendChild(noteDiv);
        noteColumn.scrollTop = noteColumn.scrollHeight;
    }
});