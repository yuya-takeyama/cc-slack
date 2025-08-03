import { describe, expect, it } from "vitest";
import {
  getSessionStatusColor,
  getSessionSummary,
  truncateSessionId,
} from "./sessionUtils";

describe("sessionUtils", () => {
  describe("getSessionStatusColor", () => {
    describe("card format", () => {
      it("should return green colors for active status", () => {
        expect(getSessionStatusColor("active")).toBe(
          "text-green-600 bg-green-100",
        );
      });

      it("should return blue colors for completed status", () => {
        expect(getSessionStatusColor("completed")).toBe(
          "text-blue-600 bg-blue-100",
        );
      });

      it("should return red colors for failed status", () => {
        expect(getSessionStatusColor("failed")).toBe("text-red-600 bg-red-100");
      });

      it("should return gray colors for unknown status", () => {
        expect(getSessionStatusColor("unknown")).toBe(
          "text-gray-600 bg-gray-100",
        );
      });

      it("should use card format by default", () => {
        expect(getSessionStatusColor("active", "card")).toBe(
          getSessionStatusColor("active"),
        );
      });
    });

    describe("table format", () => {
      it("should return green colors for active status", () => {
        expect(getSessionStatusColor("active", "table")).toBe(
          "bg-green-100 text-green-800",
        );
      });

      it("should return blue colors for completed status", () => {
        expect(getSessionStatusColor("completed", "table")).toBe(
          "bg-blue-100 text-blue-800",
        );
      });

      it("should return red colors for failed status", () => {
        expect(getSessionStatusColor("failed", "table")).toBe(
          "bg-red-100 text-red-800",
        );
      });

      it("should return gray colors for unknown status", () => {
        expect(getSessionStatusColor("unknown", "table")).toBe(
          "bg-gray-100 text-gray-800",
        );
      });
    });

    it("should throw error for unknown format", () => {
      expect(() => getSessionStatusColor("active", "invalid")).toThrow(
        "Unknown format: invalid",
      );
    });
  });

  describe("truncateSessionId", () => {
    it("should truncate long session ID", () => {
      const longId = "12345678901234567890";
      expect(truncateSessionId(longId)).toBe("12345678");
    });

    it("should not truncate short session ID", () => {
      const shortId = "12345";
      expect(truncateSessionId(shortId)).toBe("12345");
    });

    it("should handle custom length", () => {
      const id = "1234567890";
      expect(truncateSessionId(id, 4)).toBe("1234");
    });

    it("should return empty string for null", () => {
      expect(truncateSessionId(null)).toBe("");
    });

    it("should return empty string for undefined", () => {
      expect(truncateSessionId(undefined)).toBe("");
    });

    it("should return empty string for non-string", () => {
      expect(truncateSessionId(123)).toBe("");
    });

    it("should handle exact length match", () => {
      const id = "12345678";
      expect(truncateSessionId(id, 8)).toBe("12345678");
    });
  });

  describe("getSessionSummary", () => {
    it("should return summary for valid session", () => {
      const session = {
        session_id: "1234567890abcdef",
        status: "active",
      };
      const summary = getSessionSummary(session);
      expect(summary.displayId).toBe("12345678");
      expect(summary.status).toBe("active");
      expect(summary.statusColor).toBe("text-green-600 bg-green-100");
    });

    it("should handle session without status", () => {
      const session = {
        session_id: "1234567890abcdef",
      };
      const summary = getSessionSummary(session);
      expect(summary.displayId).toBe("12345678");
      expect(summary.status).toBe("unknown");
      expect(summary.statusColor).toBe("text-gray-600 bg-gray-100");
    });

    it("should handle null session", () => {
      const summary = getSessionSummary(null);
      expect(summary.displayId).toBe("");
      expect(summary.status).toBe("unknown");
      expect(summary.statusColor).toBe("text-gray-600 bg-gray-100");
    });

    it("should handle undefined session", () => {
      const summary = getSessionSummary(undefined);
      expect(summary.displayId).toBe("");
      expect(summary.status).toBe("unknown");
      expect(summary.statusColor).toBe("text-gray-600 bg-gray-100");
    });
  });
});
