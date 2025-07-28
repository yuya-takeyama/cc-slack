interface FormatOptions {
  format?: "short" | "medium" | "full";
  relative?: boolean;
  includeTime?: boolean;
  fallback?: string;
}

export const formatDateTime = (
  dateString: string | null | undefined,
  options: FormatOptions = {},
): string => {
  if (!dateString) return options.fallback || "N/A";

  const date = new Date(dateString);

  if (Number.isNaN(date.getTime())) {
    return options.fallback || "Invalid date";
  }

  const { format = "full", relative = false, includeTime = true } = options;

  if (relative) {
    return getRelativeTime(date);
  }

  switch (format) {
    case "short":
      return formatShort(date, includeTime);
    case "medium":
      return formatMedium(date, includeTime);
    default:
      return formatFull(date, includeTime);
  }
};

const getRelativeTime = (date: Date): string => {
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSecs = Math.floor(diffMs / 1000);
  const diffMins = Math.floor(diffSecs / 60);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffSecs < 60) {
    return "just now";
  } else if (diffMins < 60) {
    return `${diffMins} minute${diffMins === 1 ? "" : "s"} ago`;
  } else if (diffHours < 24) {
    return `${diffHours} hour${diffHours === 1 ? "" : "s"} ago`;
  } else if (diffDays < 7) {
    return `${diffDays} day${diffDays === 1 ? "" : "s"} ago`;
  } else if (diffDays < 30) {
    const weeks = Math.floor(diffDays / 7);
    return `${weeks} week${weeks === 1 ? "" : "s"} ago`;
  } else if (diffDays < 365) {
    const months = Math.floor(diffDays / 30);
    return `${months} month${months === 1 ? "" : "s"} ago`;
  } else {
    const years = Math.floor(diffDays / 365);
    return `${years} year${years === 1 ? "" : "s"} ago`;
  }
};

const formatShort = (date: Date, includeTime: boolean): string => {
  const options: Intl.DateTimeFormatOptions = {
    month: "numeric",
    day: "numeric",
  };

  if (includeTime) {
    options.hour = "2-digit";
    options.minute = "2-digit";
  }

  return date.toLocaleString("en-US", options);
};

const formatMedium = (date: Date, includeTime: boolean): string => {
  const options: Intl.DateTimeFormatOptions = {
    month: "short",
    day: "numeric",
    year: "numeric",
  };

  if (includeTime) {
    options.hour = "2-digit";
    options.minute = "2-digit";
  }

  return date.toLocaleString("en-US", options);
};

const formatFull = (date: Date, includeTime: boolean): string => {
  const options: Intl.DateTimeFormatOptions = {
    weekday: "short",
    year: "numeric",
    month: "short",
    day: "numeric",
  };

  if (includeTime) {
    options.hour = "2-digit";
    options.minute = "2-digit";
    options.second = "2-digit";
  }

  return date.toLocaleString("en-US", options);
};

export const formatDateRange = (
  startDate: string | null | undefined,
  endDate: string | null | undefined,
  options: FormatOptions = {},
): string => {
  const start = formatDateTime(startDate, options);

  if (!endDate) {
    return `${start} - Present`;
  }

  const end = formatDateTime(endDate, options);
  return `${start} - ${end}`;
};

export const formatDuration = (
  startDate: string | null | undefined,
  endDate?: string | null | undefined,
): string => {
  if (!startDate) return "N/A";

  const start = new Date(startDate);
  const end = endDate ? new Date(endDate) : new Date();

  const diffMs = end.getTime() - start.getTime();
  const diffSecs = Math.floor(diffMs / 1000);
  const diffMins = Math.floor(diffSecs / 60);
  const diffHours = Math.floor(diffMins / 60);

  if (diffMins < 1) {
    return `${diffSecs}s`;
  } else if (diffHours < 1) {
    return `${diffMins}m`;
  } else {
    const mins = diffMins % 60;
    return mins > 0 ? `${diffHours}h ${mins}m` : `${diffHours}h`;
  }
};
