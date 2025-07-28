type SessionStatus = "active" | "completed" | "failed" | "unknown";
type FormatType = "card" | "table";

interface Session {
  session_id?: string;
  status?: SessionStatus;
  initial_prompt?: string;
}

interface SessionSummary {
  displayId: string;
  status: SessionStatus;
  statusColor: string;
}

export const getSessionStatusColor = (
  status: SessionStatus,
  format: FormatType = "card",
): string => {
  if (format === "card") {
    switch (status) {
      case "active":
        return "text-green-600 bg-green-100";
      case "completed":
        return "text-blue-600 bg-blue-100";
      case "failed":
        return "text-red-600 bg-red-100";
      default:
        return "text-gray-600 bg-gray-100";
    }
  }

  if (format === "table") {
    switch (status) {
      case "active":
        return "bg-green-100 text-green-800";
      case "completed":
        return "bg-blue-100 text-blue-800";
      case "failed":
        return "bg-red-100 text-red-800";
      default:
        return "bg-gray-100 text-gray-800";
    }
  }

  throw new Error(`Unknown format: ${format}`);
};

export const truncateSessionId = (
  sessionId: unknown,
  length: number = 8,
): string => {
  if (!sessionId || typeof sessionId !== "string") {
    return "";
  }

  if (sessionId.length <= length) {
    return sessionId;
  }

  return `${sessionId.substring(0, length)}...`;
};

export const truncatePrompt = (
  prompt: unknown,
  length: number = 100,
): string => {
  if (!prompt || typeof prompt !== "string") {
    return "";
  }

  const cleanPrompt = prompt.trim().replace(/\s+/g, " ");
  
  if (cleanPrompt.length <= length) {
    return cleanPrompt;
  }

  return `${cleanPrompt.substring(0, length)}...`;
};

export const getSessionSummary = (
  session: Session | null | undefined,
): SessionSummary => {
  if (!session) {
    return {
      displayId: "",
      status: "unknown",
      statusColor: getSessionStatusColor("unknown"),
    };
  }

  return {
    displayId: truncateSessionId(session.session_id),
    status: (session.status || "unknown") as SessionStatus,
    statusColor: getSessionStatusColor(
      (session.status || "unknown") as SessionStatus,
    ),
  };
};
