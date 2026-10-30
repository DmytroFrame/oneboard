import { useSyncExternalStore } from "react";
import { api } from "./api";
import type { BoardSocket } from "./realtime";

/**
 * Voice rooms: one per board, like a voice channel in Discord. Audio goes peer to peer over WebRTC
 * (a full mesh: every participant sends their microphone to every other one). The server only tracks
 * who is in which room and relays signaling over the user channel — the WebSocket every tab keeps
 * open — so a call survives moving between pages.
 *
 * Signaling uses "perfect negotiation": either side may (re)negotiate at any time, and when both
 * send an offer at once, the "polite" side (the lower connection id) yields.
 */

export interface VoiceParticipant {
  connId: string;
  userId: number;
  name: string;
  muted: boolean;
}

export type CallStatus = "joining" | "connected" | "reconnecting";

/** Why the user isn't in a call any more, or why joining failed; shown by the UI and then cleared. */
export type VoiceProblem =
  | "micDenied" // the browser or the user refused microphone access
  | "noMic"
  | "micFailed"
  | "insecure" // getUserMedia needs HTTPS
  | "elsewhere" // joined from another tab or board
  | "revoked" // lost access to the project
  | string; // a server error code (errors.<code>)

export interface VoiceState {
  /** Non-empty rooms of the user's projects. */
  rooms: Record<number, VoiceParticipant[]>;
  call: { projectId: number; status: CallStatus; muted: boolean } | null;
  /** Connection ids of participants talking right now ("self" for this tab). */
  speaking: ReadonlySet<string>;
  /** WebRTC connection state per remote participant. */
  links: Record<string, RTCPeerConnectionState>;
  problem: { projectId: number; reason: VoiceProblem } | null;
  devices: MediaDeviceInfo[];
  deviceId: string;
}

type RoomChange = (projectId: number, joined: VoiceParticipant[], left: VoiceParticipant[]) => void;

// ---------- preferences ----------

const PREFS_KEY = "oneboard:voice";

function loadPrefs(): { muted: boolean; deviceId: string } {
  try {
    const p = JSON.parse(localStorage.getItem(PREFS_KEY) ?? "{}");
    return { muted: !!p.muted, deviceId: typeof p.deviceId === "string" ? p.deviceId : "" };
  } catch {
    return { muted: false, deviceId: "" };
  }
}

function savePrefs(p: { muted: boolean; deviceId: string }) {
  try {
    localStorage.setItem(PREFS_KEY, JSON.stringify(p));
  } catch {
    // private mode: preferences just aren't remembered
  }
}

// ---------- one remote participant ----------

class Link {
  readonly pc: RTCPeerConnection;
  private makingOffer = false;
  private ignoreOffer = false;
  private audio: HTMLAudioElement;
  stream: MediaStream | null = null;

  constructor(
    readonly connId: string,
    private polite: boolean,
    iceServers: RTCIceServer[],
    track: MediaStreamTrack | null,
    local: MediaStream | null,
    private send: (signal: unknown) => void,
    private onState: (state: RTCPeerConnectionState) => void,
    private onStream: (stream: MediaStream) => void,
  ) {
    this.pc = new RTCPeerConnection({ iceServers });
    this.audio = document.createElement("audio");
    this.audio.autoplay = true;
    this.audio.setAttribute("playsinline", "");
    this.audio.dataset.voicePeer = connId;
    document.body.appendChild(this.audio);

    if (track && local) this.pc.addTrack(track, local);
    else this.pc.addTransceiver("audio", { direction: "recvonly" });

    this.pc.onnegotiationneeded = async () => {
      try {
        this.makingOffer = true;
        await this.pc.setLocalDescription();
        this.send({ description: this.pc.localDescription });
      } catch (err) {
        console.warn("voice: offer failed", err);
      } finally {
        this.makingOffer = false;
      }
    };
    this.pc.onicecandidate = ({ candidate }) => {
      if (candidate) this.send({ candidate });
    };
    this.pc.ontrack = ({ streams, track: remote }) => {
      const stream = streams[0] ?? new MediaStream([remote]);
      if (this.stream === stream) return;
      this.stream = stream;
      this.audio.srcObject = stream;
      void this.audio.play().catch(() => {}); // blocked autoplay is retried on the next click (see resumeAudio)
      this.onStream(stream);
    };
    this.pc.onconnectionstatechange = () => {
      const state = this.pc.connectionState;
      this.onState(state);
      // A network change (Wi-Fi ↔ mobile): gather new candidates instead of dropping the link.
      if (state === "failed") this.pc.restartIce();
    };
  }

  async signal(msg: { description?: RTCSessionDescriptionInit; candidate?: RTCIceCandidateInit }) {
    try {
      if (msg.description) {
        const collision = msg.description.type === "offer" && (this.makingOffer || this.pc.signalingState !== "stable");
        this.ignoreOffer = !this.polite && collision;
        if (this.ignoreOffer) return;
        await this.pc.setRemoteDescription(msg.description);
        if (msg.description.type === "offer") {
          await this.pc.setLocalDescription();
          this.send({ description: this.pc.localDescription });
        }
      } else if (msg.candidate) {
        try {
          await this.pc.addIceCandidate(msg.candidate);
        } catch (err) {
          if (!this.ignoreOffer) throw err;
        }
      }
    } catch (err) {
      console.warn("voice: signaling failed", err);
    }
  }

  replaceTrack(track: MediaStreamTrack, stream: MediaStream) {
    const sender = this.pc.getSenders().find((s) => s.track?.kind === "audio" || s.track === null);
    if (sender) void sender.replaceTrack(track);
    else this.pc.addTrack(track, stream);
  }

  playAgain() {
    if (this.audio.paused && this.audio.srcObject) void this.audio.play().catch(() => {});
  }

  close() {
    this.pc.onconnectionstatechange = null;
    this.pc.close();
    this.audio.srcObject = null;
    this.audio.remove();
  }
}

// ---------- the controller ----------

const SPEAKING_LEVEL = 0.035; // RMS of the waveform (0..1) above which someone is talking
const SPEAKING_HOLD = 350; // ms to keep the indicator on through short pauses

class VoiceController {
  private state: VoiceState = {
    rooms: {},
    call: null,
    speaking: new Set(),
    links: {},
    problem: null,
    devices: [],
    deviceId: loadPrefs().deviceId,
  };
  private listeners = new Set<() => void>();
  private roomListeners = new Set<RoomChange>();

  private socket: BoardSocket | null = null;
  private offSocket: (() => void)[] = [];

  private local: MediaStream | null = null;
  private iceServers: RTCIceServer[] = [];
  private links = new Map<string, Link>();
  private joinedSent = false; // voice.join was sent on the current socket connection

  private audioCtx: AudioContext | null = null;
  private meters = new Map<string, { analyser: AnalyserNode; buf: Uint8Array<ArrayBuffer>; lastLoud: number }>();
  private meterTimer = 0;

  // ----- store -----

  get = () => this.state;

  subscribe = (fn: () => void) => {
    this.listeners.add(fn);
    return () => {
      this.listeners.delete(fn);
    };
  };

  /** Someone joined or left a room of one of the user's projects (never reports the user's own tab). */
  onRoomChange(fn: RoomChange) {
    this.roomListeners.add(fn);
    return () => {
      this.roomListeners.delete(fn);
    };
  }

  private set(patch: Partial<VoiceState>) {
    this.state = { ...this.state, ...patch };
    this.listeners.forEach((fn) => fn());
  }

  // ----- wiring to the user channel -----

  attach(socket: BoardSocket) {
    this.detach();
    this.socket = socket;
    this.offSocket = [
      socket.on("welcome", () => {
        void this.loadRooms();
        // A new connection id: rejoin the call we were in (the old participant is gone on the server).
        this.joinedSent = false;
        if (this.state.call) {
          this.closeLinks();
          this.sendJoin();
        }
      }),
      socket.on("disconnect", () => {
        this.joinedSent = false;
        if (this.state.call) this.set({ call: { ...this.state.call, status: "reconnecting" } });
      }),
      socket.on("user", (m) => {
        if (m.event?.type === "voice") this.updateRoom(m.event.projectId, m.event.participants ?? []);
      }),
      socket.on("voice.joined", (m) => {
        const call = this.state.call;
        if (!call || call.projectId !== m.projectId) return;
        this.set({ call: { ...call, status: "connected" } });
        this.updateRoom(m.projectId, m.participants);
        // Newcomers dial everyone already there.
        for (const p of m.participants as VoiceParticipant[]) if (p.connId !== socket.connId) this.link(p.connId);
        this.sound("join");
      }),
      socket.on("voice.error", (m) => {
        if (this.state.call?.projectId !== m.projectId) return;
        this.teardown();
        this.set({ problem: { projectId: m.projectId, reason: m.code } });
      }),
      socket.on("voice.left", (m) => {
        if (this.state.call?.projectId !== m.projectId) return;
        this.teardown();
        this.set({ problem: { projectId: m.projectId, reason: m.reason } });
      }),
      socket.on("voice.signal", (m) => {
        if (!this.state.call || this.state.call.status === "joining") return;
        void (this.links.get(m.from) ?? this.link(m.from)).signal(m.signal ?? {});
      }),
    ];
    if (socket.ready) void this.loadRooms();
  }

  detach() {
    if (this.state.call) this.leave();
    this.offSocket.forEach((off) => off());
    this.offSocket = [];
    this.socket = null;
    this.set({ rooms: {} });
  }

  private async loadRooms() {
    try {
      const { rooms } = await api.voiceRooms();
      const next: Record<number, VoiceParticipant[]> = {};
      for (const [id, ps] of Object.entries(rooms)) next[Number(id)] = ps;
      this.set({ rooms: next });
    } catch {
      // the next room event fills it in
    }
  }

  private updateRoom(projectId: number, participants: VoiceParticipant[]) {
    const before = this.state.rooms[projectId] ?? [];
    const rooms = { ...this.state.rooms };
    if (participants.length) rooms[projectId] = participants;
    else delete rooms[projectId];
    this.set({ rooms });

    const me = this.socket?.connId;
    const has = (list: VoiceParticipant[], p: VoiceParticipant) => list.some((x) => x.connId === p.connId);
    // A user switching tabs shows up as leave + join of the same person: report neither.
    const joined = participants.filter((p) => p.connId !== me && !has(before, p) && !before.some((b) => b.userId === p.userId));
    const left = before.filter((p) => p.connId !== me && !has(participants, p) && !participants.some((x) => x.userId === p.userId));

    if (this.state.call?.projectId === projectId && this.state.call.status === "connected") {
      for (const connId of [...this.links.keys()]) if (!participants.some((p) => p.connId === connId)) this.unlink(connId);
      if (joined.length) this.sound("join");
      if (left.length) this.sound("leave");
    }
    if (joined.length || left.length) this.roomListeners.forEach((fn) => fn(projectId, joined, left));
  }

  // ----- actions -----

  async join(projectId: number) {
    const current = this.state.call;
    if (current?.projectId === projectId) return;
    if (current) this.leave(false);

    const prefs = loadPrefs();
    this.set({ call: { projectId, status: "joining", muted: prefs.muted }, problem: null });
    this.ensureAudio();

    if (!navigator.mediaDevices?.getUserMedia) return this.fail(projectId, "insecure");
    try {
      const [stream, ice] = await Promise.all([this.openMic(prefs.deviceId), api.voiceIce().catch(() => ({ iceServers: [] }))]);
      if (this.state.call?.projectId !== projectId) {
        stream.getTracks().forEach((t) => t.stop()); // left while the permission prompt was open
        return;
      }
      this.local = stream;
      this.iceServers = ice.iceServers;
      this.applyMute(prefs.muted);
      this.meter("self", stream);
      void this.refreshDevices();
      this.sendJoin();
    } catch (err) {
      const name = (err as DOMException).name;
      this.fail(projectId, name === "NotAllowedError" || name === "SecurityError" ? "micDenied" : name === "NotFoundError" ? "noMic" : "micFailed");
    }
  }

  leave(withSound = true) {
    if (!this.state.call) return;
    this.socket?.send({ type: "voice.leave" });
    if (withSound) this.sound("leave");
    this.teardown();
  }

  toggleMute() {
    const call = this.state.call;
    if (!call) return;
    const muted = !call.muted;
    this.applyMute(muted);
    savePrefs({ ...loadPrefs(), muted });
    this.set({ call: { ...call, muted } });
    this.socket?.send({ type: "voice.mute", muted });
  }

  async setDevice(deviceId: string) {
    savePrefs({ ...loadPrefs(), deviceId });
    this.set({ deviceId });
    if (!this.local) return;
    try {
      const stream = await this.openMic(deviceId);
      const track = stream.getAudioTracks()[0];
      this.local.getTracks().forEach((t) => t.stop());
      this.local = stream;
      this.applyMute(this.state.call?.muted ?? false);
      for (const link of this.links.values()) link.replaceTrack(track, stream);
      this.meter("self", stream);
    } catch {
      this.set({ problem: { projectId: this.state.call?.projectId ?? 0, reason: "micFailed" } });
    }
  }

  clearProblem() {
    if (this.state.problem) this.set({ problem: null });
  }

  /** Browsers may block audio that starts without a click; any later click lets it play. */
  resumeAudio() {
    void this.audioCtx?.resume();
    for (const link of this.links.values()) link.playAgain();
  }

  // ----- internals -----

  private sendJoin() {
    const call = this.state.call;
    if (!call || !this.local || !this.socket?.ready || this.joinedSent) return;
    this.joinedSent = true;
    this.socket.send({ type: "voice.join", projectId: call.projectId, muted: call.muted });
  }

  private fail(projectId: number, reason: VoiceProblem) {
    this.teardown();
    this.set({ problem: { projectId, reason } });
  }

  private openMic(deviceId: string) {
    return navigator.mediaDevices.getUserMedia({
      audio: {
        deviceId: deviceId ? { ideal: deviceId } : undefined,
        echoCancellation: true,
        noiseSuppression: true,
        autoGainControl: true,
      },
      video: false,
    });
  }

  private applyMute(muted: boolean) {
    this.local?.getAudioTracks().forEach((t) => (t.enabled = !muted));
  }

  private async refreshDevices() {
    try {
      const all = await navigator.mediaDevices.enumerateDevices();
      this.set({ devices: all.filter((d) => d.kind === "audioinput" && d.deviceId) });
    } catch {
      // keep the old list
    }
  }

  private onDeviceChange = () => void this.refreshDevices();

  private link(connId: string): Link {
    const me = this.socket?.connId ?? "";
    const link = new Link(
      connId,
      me < connId,
      this.iceServers,
      this.local?.getAudioTracks()[0] ?? null,
      this.local,
      (signal) => this.socket?.send({ type: "voice.signal", to: connId, signal }),
      (state) => this.set({ links: { ...this.state.links, [connId]: state } }),
      (stream) => this.meter(connId, stream),
    );
    this.links.set(connId, link);
    this.set({ links: { ...this.state.links, [connId]: "new" } });
    return link;
  }

  private unlink(connId: string) {
    this.links.get(connId)?.close();
    this.links.delete(connId);
    this.meters.delete(connId);
    const links = { ...this.state.links };
    delete links[connId];
    this.set({ links });
  }

  private closeLinks() {
    for (const connId of [...this.links.keys()]) this.unlink(connId);
  }

  private teardown() {
    this.closeLinks();
    this.local?.getTracks().forEach((t) => t.stop());
    this.local = null;
    this.meters.clear();
    this.joinedSent = false;
    clearInterval(this.meterTimer);
    this.meterTimer = 0;
    navigator.mediaDevices?.removeEventListener?.("devicechange", this.onDeviceChange);
    this.set({ call: null, links: {}, speaking: new Set() });
  }

  // ----- audio levels and sounds -----

  private ensureAudio() {
    if (!this.audioCtx) {
      const Ctx = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
      if (Ctx) this.audioCtx = new Ctx();
    }
    void this.audioCtx?.resume();
    navigator.mediaDevices?.addEventListener?.("devicechange", this.onDeviceChange);
  }

  private meter(id: string, stream: MediaStream) {
    const ctx = this.audioCtx;
    if (!ctx) return;
    const analyser = ctx.createAnalyser();
    analyser.fftSize = 512;
    ctx.createMediaStreamSource(stream).connect(analyser); // not to the speakers: the <audio> element plays it
    this.meters.set(id, { analyser, buf: new Uint8Array(analyser.fftSize), lastLoud: 0 });
    if (!this.meterTimer) this.meterTimer = window.setInterval(() => this.measure(), 120);
  }

  private measure() {
    const now = performance.now();
    const speaking = new Set<string>();
    for (const [id, m] of this.meters) {
      m.analyser.getByteTimeDomainData(m.buf);
      let sum = 0;
      for (const v of m.buf) sum += ((v - 128) / 128) ** 2;
      if (Math.sqrt(sum / m.buf.length) > SPEAKING_LEVEL) m.lastLoud = now;
      const muted = id === "self" ? this.state.call?.muted : this.participant(id)?.muted;
      if (!muted && now - m.lastLoud < SPEAKING_HOLD) speaking.add(id);
    }
    const prev = this.state.speaking;
    if (speaking.size !== prev.size || [...speaking].some((id) => !prev.has(id))) this.set({ speaking });
  }

  private participant(connId: string) {
    const call = this.state.call;
    return call ? this.state.rooms[call.projectId]?.find((p) => p.connId === connId) : undefined;
  }

  /** A short two-note chime: rising when someone joins, falling when someone leaves. */
  private sound(kind: "join" | "leave") {
    const ctx = this.audioCtx;
    if (!ctx || ctx.state !== "running") return;
    const notes = kind === "join" ? [587, 880] : [880, 523];
    const start = ctx.currentTime;
    notes.forEach((freq, i) => {
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = "sine";
      osc.frequency.value = freq;
      const t = start + i * 0.11;
      gain.gain.setValueAtTime(0, t);
      gain.gain.linearRampToValueAtTime(0.08, t + 0.015);
      gain.gain.exponentialRampToValueAtTime(0.0001, t + 0.22);
      osc.connect(gain).connect(ctx.destination);
      osc.start(t);
      osc.stop(t + 0.24);
    });
  }
}

export const voice = new VoiceController();

export function useVoice(): VoiceState {
  return useSyncExternalStore(voice.subscribe, voice.get);
}
