package logs

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func ServeLogWithRange(c *fiber.Ctx, dagRunId int, podName string, logFetcher LogFetcher) error {
	logFileKey := fmt.Sprintf("/%v/%s-log.txt", dagRunId, podName)

	exists, fileSize, err := logFetcher.LogFileExists(&logFileKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			fiber.Map{
				"message": "unable to check if file exists",
			},
		)
	}

	if !exists {
		return c.Status(fiber.StatusNoContent).JSON(
			fiber.Map{
				"message": "file was empty or did not exist",
			},
		)
	}

	// Set headers
	c.Set("Content-Type", "text/plain")

	// Handle range requests
	rangeHeader := c.Get("Range")
	if rangeHeader != "" {
		// Parse the range header (e.g., "bytes=0-1023")
		byteRange := strings.TrimPrefix(rangeHeader, "bytes=")
		parts := strings.Split(byteRange, "-")

		start, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid range start")
		}

		end := fileSize - 1 // Default to the end of the file
		if len(parts) > 1 && parts[1] != "" {
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).SendString("Invalid range end")
			}
		}

		// Ensure the range is valid
		if start > end || start < 0 || end >= fileSize {
			return c.Status(fiber.StatusRequestedRangeNotSatisfiable).SendString(fmt.Sprintf("Invalid byte range %d-%d", start, end))
		}

		reader, err := logFetcher.RangeFetchLogs(&logFileKey, start, end)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "unable to fetch file range reader",
			})
		}

		defer reader.Close()

		// Set the content-range header and serve the requested range
		c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
		c.Status(fiber.StatusPartialContent)

		// Stream the requested range
		if _, err := io.Copy(c, reader); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error streaming file",
			})
		}

		return nil
	}

	reader, err := logFetcher.FetchLogs(&logFileKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to fetch file reader",
		})
	}

	defer reader.Close()

	// Stream the full file
	if _, err := io.Copy(c, reader); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error streaming full file")
	}

	return nil
}
