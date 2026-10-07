import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { getAccessToken } from "../api/client";
import { schoolApi, type ChatMessage } from "../api/school";
import { useChatUiStore } from "../store/chatUiStore";

type IncomingEvent =
  | { type: "message"; message: ChatMessage }
  | { type: "opencode"; session_id: number; message: { id: number; role: string; content: string; created_at: string } };

/**
 * Keeps one live WebSocket connection open for the lifetime of the app shell
 * and pushes incoming chat messages straight into the React Query cache, so
 * open conversations update instantly and channel/unread badges stay fresh
 * without polling.
 */
export function useChatSocket() {
  const client = useQueryClient();
  const socketRef = useRef<WebSocket | undefined>(undefined);

  useEffect(() => {
    let cancelled = false;
    let retryTimer: ReturnType<typeof setTimeout>;

    const connect = () => {
      const token = getAccessToken();
      if (!token || cancelled) return;
      const base = import.meta.env.VITE_API_BASE ?? "http://localhost:8080";
      const wsUrl = `${base.replace(/^http/, "ws")}/ws?token=${encodeURIComponent(token)}`;
      const socket = new WebSocket(wsUrl);
      socketRef.current = socket;

      socket.onmessage = (event) => {
        let payload: IncomingEvent;
        try {
          payload = JSON.parse(event.data);
        } catch {
          return;
        }
        // OpenCode-Antworten (KI-Lernchat) an die AiChatPage weiterreichen —
        // sie hört auf "smarttable:opencode" und pflegt sie in den Verlauf ein.
        if (payload.type === "opencode") {
          window.dispatchEvent(new MessageEvent("smarttable:opencode", { data: event.data }));
          return;
        }
        if (payload.type !== "message") return;
        const message = payload.message;
        client.setQueryData<ChatMessage[]>(["messages", message.channel_id], (prev) =>
          prev?.some((entry) => entry.id === message.id) ? prev : [message, ...(prev ?? [])]
        );
        if (message.channel_id === useChatUiStore.getState().activeChannelId) {
          // The conversation is open on screen right now - tell the backend
          // it was read immediately instead of leaving it counted as unread
          // until the user switches away and back.
          schoolApi.markChannelRead(message.channel_id).then(() => client.invalidateQueries({ queryKey: ["channels"] }));
        } else {
          client.invalidateQueries({ queryKey: ["channels"] });
        }
      };

      socket.onclose = () => {
        if (!cancelled) retryTimer = setTimeout(connect, 3000);
      };
    };

    connect();
    return () => {
      cancelled = true;
      clearTimeout(retryTimer);
      socketRef.current?.close();
    };
  }, [client]);
}
