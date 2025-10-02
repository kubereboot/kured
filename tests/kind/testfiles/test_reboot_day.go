package main

import (
	"fmt"
	"log"
	"time"
)

// simulateKuredReboot simulates Kured's reboot check for a given day
func simulateKuredReboot(today int, rebootDayOfMonth int, dryRun bool) {
	if rebootDayOfMonth != 0 && today != rebootDayOfMonth {
		log.Printf("Day %d: Skipping reboot (reboot-day-of-month=%d)\n", today, rebootDayOfMonth)
		return
	}

	if dryRun {
		log.Printf("Day %d: Reboot required but dry-run enabled\n", today)
	} else {
		log.Printf("Day %d: Reboot triggered!\n", today)
	}
}

func main() {
	// Set dryRun mode
	dryRun := true

	// Set the reboot-day-of-month to test
	rebootDayOfMonth := 15

	fmt.Printf("Testing reboot-day-of-month=%d for all days of the month (dry-run=%v)\n\n", rebootDayOfMonth, dryRun)

	// Simulate every day of the month
	for day := 1; day <= 31; day++ {
		simulateKuredReboot(day, rebootDayOfMonth, dryRun)
		time.Sleep(50 * time.Millisecond) // small delay for readability
	}

	fmt.Println("\nTest completed!")
}
