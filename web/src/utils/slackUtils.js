export const buildSlackThreadUrl = (thread) => {
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

export const formatThreadTimestamp = (threadTs) => {
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

export const getChannelDisplayName = (thread) => {
  if (!thread) {
    return "";
  }

  return thread.channel_name || thread.channel_id || "";
};

export const getThreadDisplayName = (thread) => {
  if (!thread) {
    return "";
  }

  return thread.thread_time || thread.thread_ts || "";
};
