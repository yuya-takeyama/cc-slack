export const getSessionStatusColor = (status, format = "card") => {
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

export const truncateSessionId = (sessionId, length = 8) => {
  if (!sessionId || typeof sessionId !== "string") {
    return "";
  }

  if (sessionId.length <= length) {
    return sessionId;
  }

  return `${sessionId.substring(0, length)}...`;
};

export const getSessionSummary = (session) => {
  if (!session) {
    return {
      displayId: "",
      status: "unknown",
      statusColor: getSessionStatusColor("unknown"),
    };
  }

  return {
    displayId: truncateSessionId(session.session_id),
    status: session.status || "unknown",
    statusColor: getSessionStatusColor(session.status),
  };
};
