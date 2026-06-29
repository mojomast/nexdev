# Website with Game Engine - 10 Playable Games

**Task:** Design and build a website with a game engine that includes 10 playable games.

**Constraint:** Use **nothing but MCP calls** to geoffrussy for all operations.

---

## Requirements

### Application Features
- **Website**: Modern, responsive web interface
- **Game Engine**: JavaScript/Canvas-based game engine
- **10 Playable Games**: Complete, working games with:
  - Different genres (puzzle, arcade, strategy, etc.)
  - Score tracking
  - High score persistence
  - Game controls (keyboard, mouse, touch)
- **Game Selection**: Menu to choose between games
- **Game Management**: Start, pause, restart, quit functionality
- **Responsive Design**: Works on desktop, tablet, and mobile

### Mandatory MCP Usage

**ALL** operations must use geoffrussy MCP server:

1. **Project Setup**
   - Use `geoffrussy init` (or verify project is initialized via `get_status` tool)
   - Use `create_checkpoint` to save progress before each major step
   - Use `list_checkpoints` to track progress

2. **Design Phase**
   - Use `create_checkpoint` - checkpoint: "design-start"
   - Document architecture decisions in a file (tracked by git)
   - Use `create_checkpoint` - checkpoint: "design-complete"

3. **Development Phase**
   - Use `create_checkpoint` - checkpoint: "development-start"
   - Write all code using MCP tool calls to verify project state
   - Use `get_status` and `get_stats` periodically to track progress
   - Use `create_checkpoint` - checkpoint: "development-complete"

4. **Testing Phase**
   - Use `create_checkpoint` - checkpoint: "testing-start"
   - Test website deployment
   - Test all 10 games for functionality
   - Test responsive design
   - Use `create_checkpoint` - checkpoint: "testing-complete"

---

## Implementation Steps (Using MCP Only)

### Step 1: Initialize and Plan
```bash
# MCP calls required:
1. initialize() - Connect to MCP server
2. tools/call get_status - Check project state
3. tools/call create_checkpoint - checkpoint: "game-website-start"
```

### Step 2: Design Architecture
```bash
# MCP calls required:
1. tools/call create_checkpoint - checkpoint: "design-start"
2. Write architecture document
3. tools/call create_checkpoint - checkpoint: "design-complete"
```

**Architecture to document:**

```
┌─────────────────────────────────┐
│      Frontend (HTML/CSS/JS)  │
└──────────────┬───────────────┘
               │
               ▼
┌──────────────────────────────────┐
│       Game Engine Core         │
│  - Game Loop Manager          │
│  - Input Handler             │
│  - Rendering Engine          │
│  - Audio Manager            │
│  - Score/High Score System   │
└──────┬──────────┬───────────┘
       │          │
   ┌───┴───┐  ┌─┴──────────────────┐
   │ Game 1 │  │   Game 2...Game 10 │
   │ (Puzzle)│  │  (Various Genres)  │
   └─────────┘  └─────────────────────┘
               │
               ▼
┌──────────────────────────────────┐
│   Local Storage / Database    │
│  - High Scores             │
│  - Game Progress           │
│  - User Settings          │
└─────────────────────────────────┘
```

### Step 3: Implement Game Engine Core
```bash
# MCP calls required:
1. tools/call create_checkpoint - checkpoint: "engine-start"
2. Implement game engine core
3. Test game engine
4. tools/call create_checkpoint - checkpoint: "engine-complete"
```

**Core Components:**
- `GameLoop` - RequestAnimationFrame-based game loop
- `InputManager` - Handle keyboard, mouse, touch
- `Renderer` - Canvas drawing functions
- `AudioManager` - Sound effects and music
- `ScoreManager` - Score tracking and high scores
- `GameStateManager` - Menu, playing, paused, game over

### Step 4: Implement Game Menu
```bash
# MCP calls required:
1. tools/call create_checkpoint - checkpoint: "menu-start"
2. Implement game selection menu
3. Test menu navigation
4. tools/call create_checkpoint - checkpoint: "menu-complete"
```

**Menu Features:**
- Display all 10 games with thumbnails
- Game descriptions and instructions
- Keyboard/touch navigation
- High score display for each game

### Step 5: Implement 10 Playable Games
```bash
# MCP calls required:
1. tools/call create_checkpoint - checkpoint: "games-start"
2. Implement all 10 games
3. Test each game
4. tools/call create_checkpoint - checkpoint: "games-complete"
```

**10 Games Required:**

1. **Snake** - Classic snake game
   - Eat food to grow
   - Avoid walls and self
   - Score: food eaten

2. **Tetris** - Block stacking game
   - Rotate and place falling blocks
   - Clear lines for points
   - Levels increase speed

3. **Pong** - Classic paddle game
   - Player vs AI
   - Ball physics
   - Score: balls past opponent

4. **Breakout** - Brick breaker
   - Paddle and ball
   - Destroy all bricks
   - Power-ups (optional)

5. **Memory Match** - Card matching game
   - Find matching pairs
   - Time limit or moves limit
   - Score: pairs found

6. **2048** - Number puzzle
   - Slide tiles to combine
   - Reach 2048
   - Score: tile values combined

7. **Asteroids** - Space shooter
   - Ship controls
   - Destroy asteroids
   - Score: asteroids destroyed

8. **Tic-Tac-Toe** - Strategy game
   - Player vs AI
   - Win/lose detection
   - Minimax AI (optional)

9. **Whack-a-Mole** - Reflex game
   - Click/tap moles
   - Time limit
   - Score: moles hit

10. **Jump** - Platformer
    - Jump over obstacles
    - Gravity physics
    - Score: distance traveled

### Step 6: Integrate and Test
```bash
# MCP calls required:
1. tools/call create_checkpoint - checkpoint: "integration-start"
2. Integrate all components
3. Test complete website
4. tools/call create_checkpoint - checkpoint: "integration-complete"
```

---

## MCP Tool Definitions to Use

You must use these geoffrussy MCP tools:

### get_status
```json
{
  "name": "get_status",
  "arguments": {"projectPath": "/path/to/project"}
}
```
Returns project status, stage, progress

### get_stats
```json
{
  "name": "get_stats",
  "arguments": {"projectPath": "/path/to/project"}
}
```
Returns token usage and cost statistics

### list_phases
```json
{
  "name": "list_phases",
  "arguments": {"projectPath": "/path/to/project"}
}
```
Returns development phases and tasks

### create_checkpoint
```json
{
  "name": "create_checkpoint",
  "arguments": {
    "projectPath": "/path/to/project",
    "name": "checkpoint-name"
  }
}
```
Creates a git tag and database checkpoint

### list_checkpoints
```json
{
  "name": "list_checkpoints",
  "arguments": {"projectPath": "/path/to/project"}
}
```
Lists all checkpoints

---

## Game Engine Architecture

### Technology Stack
- **HTML5** - Structure
- **CSS3** - Styling and animations
- **JavaScript (ES6+)** - Game logic and engine
- **Canvas API** - Rendering
- **Web Audio API** - Sound effects
- **LocalStorage** - High score persistence

### Game Engine Classes

#### GameEngine
```javascript
class GameEngine {
  constructor(canvas, context)
  start()
  stop()
  pause()
  resume()
  addGame(game)
  removeGame(id)
  getGame(id)
  setActiveGame(id)
}
```

#### Game (Base Class)
```javascript
class Game {
  constructor(name, description)
  init()
  update(deltaTime)
  draw()
  handleInput(input)
  cleanup()
  getScore()
  getHighScore()
}
```

#### InputManager
```javascript
class InputManager {
  constructor()
  onKeyDown(key)
  onKeyUp(key)
  onMouseDown(x, y)
  onMouseUp(x, y)
  onTouchStart(x, y)
  onTouchEnd(x, y)
  getState()
}
```

#### ScoreManager
```javascript
class ScoreManager {
  constructor(gameId)
  addScore(points)
  getScore()
  getHighScore()
  saveHighScore()
  loadHighScore()
  resetScore()
}
```

### File Structure

```
game-website/
├── index.html              # Main entry point
├── css/
│   └── styles.css          # All styles
├── js/
│   ├── engine.js          # Game engine core
│   ├── games.js          # All 10 game classes
│   ├── input.js          # Input handling
│   ├── score.js          # Score management
│   └── main.js          # Main application logic
├── assets/
│   ├── images/           # Game graphics
│   └── sounds/          # Audio files
└── README.md             # Documentation
```

---

## Example MCP Workflow

```bash
# Start Geoffrussy MCP server in background
geoffrussy mcp-server --project-path /path/to/project &

# Create checkpoint before starting
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_checkpoint","arguments":{"projectPath":"'"$(pwd)"'","name":"game-website-start"}}}' | \
  geoffrussy mcp-server --project-path "$(pwd)"

# Write code...

# Create checkpoint after design
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"create_checkpoint","arguments":{"projectPath":"'"$(pwd)"'","name":"design-complete"}}}' | \
  geoffrussy mcp-server --project-path "$(pwd)"

# Check status periodically
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_status","arguments":{"projectPath":"'"$(pwd)"'"}}}' | \
  geoffrussy mcp-server --project-path "$(pwd)"
```

---

## Configuration File

Create a `.env` configuration file:

```bash
# Website Configuration
SITE_TITLE=Game Arcade
SITE_AUTHOR=Your Name

# Game Engine Configuration
CANVAS_WIDTH=800
CANVAS_HEIGHT=600
TARGET_FPS=60

# Storage Configuration
USE_LOCAL_STORAGE=true
HIGH_SCORE_KEY_PREFIX=game_arena_
```

---

## Success Criteria

✅ Website loads and displays game menu
✅ Game engine core functions correctly (loop, input, rendering)
✅ All 10 games are playable and complete
✅ Each game has score tracking
✅ High scores persist between sessions
✅ Responsive design works on desktop, tablet, mobile
✅ Game menu allows selection between games
✅ Games can be paused, restarted, and quit
✅ All operations tracked via MCP tools
✅ Code is clean, documented, and follows best practices
✅ Checkpoints created at each major milestone

---

## Deliverables

1. **Source code** - Complete website with game engine and 10 games
2. **Configuration example** - .env template
3. **README.md** - Setup and usage instructions
4. **Checkpoints** - All milestones saved via MCP
5. **MCP Build Receipt** - Documentation of MCP calls made
6. **Test results** - Demonstration of all 10 games working

---

## Getting Started

1. **Start Geoffrussy MCP server** in a separate terminal:
   ```bash
   geoffrussy mcp-server --project-path /path/to/project
   ```

2. **Initialize your build process**:
   ```bash
   # MCP Call: create_checkpoint
   echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_checkpoint","arguments":{"projectPath":"...","name":"game-website-start"}}}' | \
     geoffrussy mcp-server --project-path ...
   ```

3. **Follow the implementation steps** using only MCP tool calls

4. **Verify each step** with `get_status` and `create_checkpoint`

5. **Final verification**:
   - Open website in browser
   - Navigate to game menu
   - Play each of the 10 games
   - Verify scores and high scores
   - Test on different screen sizes

---

## Important Notes

- **NO direct file operations** - All project state via MCP tools
- **NO manual checkpoints** - Use `create_checkpoint` tool
- **Document everything** - Each checkpoint should be meaningful
- **Test incrementally** - Verify each component before moving on
- **Save all MCP receipts** - Document every MCP call with timestamps

---

## Game Implementation Checklist

For each game, implement:

- [ ] Game class extending base Game class
- [ ] `init()` method to set up game state
- [ ] `update(deltaTime)` method for game logic
- [ ] `draw()` method for rendering
- [ ] `handleInput(input)` method for controls
- [ ] Score tracking via ScoreManager
- [ ] Game over detection
- [ ] Restart functionality
- [ ] Instructions/description for menu
- [ ] Thumbnail for menu (optional)

---

**Good luck! The game website will demonstrate building a complete game platform using Geoffrussy MCP for agent capabilities.**
