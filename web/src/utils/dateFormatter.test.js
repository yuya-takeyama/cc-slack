import { describe, expect, it } from "vitest";
import {
  formatDateRange,
  formatDateTime,
  formatDuration,
} from "./dateFormatter";

describe("dateFormatter", () => {
  describe("formatDateTime", () => {
    it("should return fallback for null input", () => {
      expect(formatDateTime(null)).toBe("N/A");
    });

    it("should return custom fallback for null input", () => {
      expect(formatDateTime(null, { fallback: "No date" })).toBe("No date");
    });

    it("should return fallback for invalid date", () => {
      expect(formatDateTime("invalid-date")).toBe("Invalid date");
    });

    it("should format date with default full format", () => {
      const date = "2024-01-15T10:30:45Z";
      const result = formatDateTime(date);
      expect(result).toContain("Jan");
      expect(result).toContain("15");
      expect(result).toContain("2024");
    });

    it("should format date with short format", () => {
      const date = "2024-01-15T10:30:45Z";
      const result = formatDateTime(date, { format: "short" });
      expect(result).toContain("1/15");
    });

    it("should format date with medium format", () => {
      const date = "2024-01-15T10:30:45Z";
      const result = formatDateTime(date, { format: "medium" });
      expect(result).toContain("Jan");
      expect(result).toContain("15");
      expect(result).toContain("2024");
    });
  });

  describe("formatDateRange", () => {
    it("should format date range with both dates", () => {
      const start = "2024-01-15T10:30:45Z";
      const end = "2024-01-15T11:45:30Z";
      const result = formatDateRange(start, end);
      expect(result).toContain(" - ");
    });

    it("should handle null end date", () => {
      const start = "2024-01-15T10:30:45Z";
      const result = formatDateRange(start, null);
      expect(result).toContain(" - Present");
    });
  });

  describe("formatDuration", () => {
    it("should return N/A for null start date", () => {
      expect(formatDuration(null, null)).toBe("N/A");
    });

    it("should format duration in seconds", () => {
      const start = new Date();
      const end = new Date(start.getTime() + 30 * 1000); // 30 seconds later
      const result = formatDuration(start.toISOString(), end.toISOString());
      expect(result).toBe("30s");
    });

    it("should format duration in minutes", () => {
      const start = new Date();
      const end = new Date(start.getTime() + 5 * 60 * 1000); // 5 minutes later
      const result = formatDuration(start.toISOString(), end.toISOString());
      expect(result).toBe("5m");
    });

    it("should format duration in hours and minutes", () => {
      const start = new Date();
      const end = new Date(start.getTime() + (2 * 60 + 15) * 60 * 1000); // 2 hours 15 minutes later
      const result = formatDuration(start.toISOString(), end.toISOString());
      expect(result).toBe("2h 15m");
    });

    it("should format duration in hours only", () => {
      const start = new Date();
      const end = new Date(start.getTime() + 3 * 60 * 60 * 1000); // 3 hours later
      const result = formatDuration(start.toISOString(), end.toISOString());
      expect(result).toBe("3h");
    });
  });
});
