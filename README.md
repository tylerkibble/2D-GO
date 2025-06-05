# 2D-GO

A simple 2D shooter game built with [Ebitengine (Ebiten)](https://ebitengine.org/) in Go.  
Move your player, shoot enemies, dodge bullets, and compete for the highest score!

## Features

- Keyboard and mouse controls
- Scrolling background
- Increasing difficulty (faster enemy spawns)
- Persistent high scores (per username)
- Leaderboard (top 10)
- Customizable window size (via settings)
- Simple settings and menu UI

## Controls

- **Move:** WASD or Arrow Keys
- **Shoot:** Left Mouse Button
- **Menu Navigation:** Mouse
- **Enter Username:** Type on keyboard, press `Enter` to start
- **Restart/Return to Menu:** Use on-screen buttons or `Enter`/`Escape` on death screen

## How to Run

1. **Install Go** (if not already):  
    ```
   https://go.dev/dl/
    ```
2. **Clone or Download this repository**
    ```
    https://github.com/tylerkibble/2D-GO.git
    ```
3. **Install dependencies:**  
   ```
   go mod tidy
   ```

4. **Run the game:**  
   ```
   go run main.go
   ```

   The game window will open. Enter a username and start playing!

## Saving & High Scores

- High scores are saved per username in `scores.json` in the same directory.
- The leaderboard shows the top 10 scores across all users.

## Custom Window Size

- Go to **Settings** from the menu or death screen.
- Choose a preset or enter a custom width and height (minimum 100x100).

## Dependencies

- [Ebitengine (Ebiten)](https://github.com/hajimehoshi/ebiten)
- [bitmapfont](https://github.com/hajimehoshi/bitmapfont)
- [go-text/typesetting](https://github.com/go-text/typesetting)

All dependencies are managed via Go modules.

## License

MIT License.  
See [LICENSE](LICENSE) if provided.

---

Made with Go and [Ebitengine](https://ebitengine.org/).