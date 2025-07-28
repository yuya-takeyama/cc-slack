interface Thread {
  workspace_subdomain?: string;
  channel_id?: string;
  thread_ts?: string;
  channel_name?: string;
  thread_time?: string;
}

export const buildSlackThreadUrl = (thread: Thread | null): string | null => {
  if (
    !thread ||
    !thread.workspace_subdomain ||
    !thread.channel_id ||
    !thread.thread_ts
  ) {
    return null;
  }

  const threadTsFormatted = thread.thread_ts.replace(".", "");
  return `https://${thread.workspace_subdomain}.slack.com/archives/${thread.channel_id}/p${threadTsFormatted}`;
};

export const formatThreadTimestamp = (threadTs: unknown): string => {
  if (!threadTs || typeof threadTs !== "string") {
    return "";
  }

  const parts = threadTs.split(".");
  if (parts.length !== 2) {
    return threadTs;
  }

  const timestamp = Number.parseInt(parts[0], 10);
  if (Number.isNaN(timestamp)) {
    return threadTs;
  }

  const date = new Date(timestamp * 1000);
  return date.toLocaleString();
};

export const getChannelDisplayName = (thread: Thread | null): string => {
  if (!thread) {
    return "";
  }

  return thread.channel_name || thread.channel_id || "";
};

export const getThreadDisplayName = (thread: Thread | null): string => {
  if (!thread) {
    return "";
  }

  return thread.thread_time || thread.thread_ts || "";
};
