import { describe, expect, it } from "vitest";
import {
  buildSlackThreadUrl,
  formatThreadTimestamp,
  getChannelDisplayName,
  getThreadDisplayName,
} from "./slackUtils";

describe("slackUtils", () => {
  describe("buildSlackThreadUrl", () => {
    it("should build valid Slack URL", () => {
      const thread = {
        workspace_subdomain: "myworkspace",
        channel_id: "C1234567890",
        thread_ts: "1234567890.123456",
      };
      expect(buildSlackThreadUrl(thread)).toBe(
        "https://myworkspace.slack.com/archives/C1234567890/p1234567890123456",
      );
    });

    it("should return null for null thread", () => {
      expect(buildSlackThreadUrl(null)).toBe(null);
    });

    it("should return null for missing workspace_subdomain", () => {
      const thread = {
        channel_id: "C1234567890",
        thread_ts: "1234567890.123456",
      };
      expect(buildSlackThreadUrl(thread)).toBe(null);
    });

    it("should return null for missing channel_id", () => {
      const thread = {
        workspace_subdomain: "myworkspace",
        thread_ts: "1234567890.123456",
      };
      expect(buildSlackThreadUrl(thread)).toBe(null);
    });

    it("should return null for missing thread_ts", () => {
      const thread = {
        workspace_subdomain: "myworkspace",
        channel_id: "C1234567890",
      };
      expect(buildSlackThreadUrl(thread)).toBe(null);
    });
  });

  describe("formatThreadTimestamp", () => {
    it("should format valid timestamp", () => {
      const threadTs = "1234567890.123456";
      const result = formatThreadTimestamp(threadTs);
      expect(result).toContain("2009");
    });

    it("should return empty string for null", () => {
      expect(formatThreadTimestamp(null)).toBe("");
    });

    it("should return empty string for undefined", () => {
      expect(formatThreadTimestamp(undefined)).toBe("");
    });

    it("should return empty string for non-string", () => {
      expect(formatThreadTimestamp(123)).toBe("");
    });

    it("should return original for invalid format", () => {
      expect(formatThreadTimestamp("invalid")).toBe("invalid");
    });

    it("should return original for non-numeric timestamp", () => {
      expect(formatThreadTimestamp("abc.def")).toBe("abc.def");
    });
  });

  describe("getChannelDisplayName", () => {
    it("should prefer channel_name over channel_id", () => {
      const thread = {
        channel_name: "general",
        channel_id: "C1234567890",
      };
      expect(getChannelDisplayName(thread)).toBe("general");
    });

    it("should fallback to channel_id", () => {
      const thread = {
        channel_id: "C1234567890",
      };
      expect(getChannelDisplayName(thread)).toBe("C1234567890");
    });

    it("should return empty string for null thread", () => {
      expect(getChannelDisplayName(null)).toBe("");
    });

    it("should return empty string for empty thread", () => {
      expect(getChannelDisplayName({})).toBe("");
    });
  });

  describe("getThreadDisplayName", () => {
    it("should prefer thread_time over thread_ts", () => {
      const thread = {
        thread_time: "2024-01-15 10:30:45",
        thread_ts: "1234567890.123456",
      };
      expect(getThreadDisplayName(thread)).toBe("2024-01-15 10:30:45");
    });

    it("should fallback to thread_ts", () => {
      const thread = {
        thread_ts: "1234567890.123456",
      };
      expect(getThreadDisplayName(thread)).toBe("1234567890.123456");
    });

    it("should return empty string for null thread", () => {
      expect(getThreadDisplayName(null)).toBe("");
    });

    it("should return empty string for empty thread", () => {
      expect(getThreadDisplayName({})).toBe("");
    });
  });
});
