package main

import (
	"fmt"
	"math/rand/v2"
	"sort"

	"charm.land/lipgloss/v2"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("14")).
			MarginBottom(1)

	numStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1).
			Bold(true)

	bonusStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("220")).
			Foreground(lipgloss.Color("220")).
			Padding(0, 1).
			Bold(true)

	luckyStarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("214")).
			Foreground(lipgloss.Color("214")).
			Padding(0, 1).
			Bold(true)

	powerballStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("196")).
			Foreground(lipgloss.Color("196")).
			Padding(0, 1).
			Bold(true)

	sepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Bold(true).
			Padding(0, 1)
)

func pick(n, max int) []int {
	pool := make([]int, max)
	for i := range pool {
		pool[i] = i + 1
	}
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	chosen := pool[:n]
	sort.Ints(chosen)
	return chosen
}

func balls(nums []int, style lipgloss.Style) []string {
	rendered := make([]string, len(nums))
	for i, n := range nums {
		rendered[i] = style.Render(fmt.Sprintf("%02d", n))
	}
	return rendered
}

func row(main []int, sep string, bonus []int, bonusSty lipgloss.Style) string {
	parts := balls(main, numStyle)
	parts = append(parts, sepStyle.Render(sep))
	parts = append(parts, balls(bonus, bonusSty)...)
	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

func main() {
	fmt.Println(titleStyle.Render("UK National Lottery"))
	fmt.Println(row(pick(6, 59), "+", pick(1, 59), bonusStyle))
	fmt.Println()

	fmt.Println(titleStyle.Render("EuroMillions"))
	fmt.Println(row(pick(5, 50), "★", pick(2, 12), luckyStarStyle))
	fmt.Println()

	fmt.Println(titleStyle.Render("US Powerball"))
	fmt.Println(row(pick(5, 69), "+", pick(1, 26), powerballStyle))
}
