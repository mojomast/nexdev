class ArcadeGame {
    constructor(definition, engine) {
        this.id = definition.id;
        this.name = definition.name;
        this.description = definition.description;
        this.definition = definition;
        this.engine = engine;

        this.player = null;
        this.collectibles = [];
        this.hazards = [];
        this.timeRemaining = definition.duration;
        this.isGameOver = false;
        this.gameOverReason = "";
        this.onGameOver = null;
    }

    init() {
        this.player = {
            x: CONFIG.CANVAS_WIDTH / 2,
            y: CONFIG.CANVAS_HEIGHT / 2,
            r: 14,
            speed: this.definition.playerSpeed
        };
        this.collectibles = [];
        this.hazards = [];
        this.timeRemaining = this.definition.duration;
        this.isGameOver = false;
        this.gameOverReason = "";

        const scoreManager = this.engine.getScoreManager();
        if (scoreManager) {
            scoreManager.resetScore();
        }

        for (let i = 0; i < this.definition.collectibleCount; i += 1) {
            this.collectibles.push(this.createCollectible());
        }

        for (let i = 0; i < this.definition.hazardCount; i += 1) {
            this.hazards.push(this.createHazard());
        }
    }

    createCollectible() {
        return {
            x: 30 + Math.random() * (CONFIG.CANVAS_WIDTH - 60),
            y: 30 + Math.random() * (CONFIG.CANVAS_HEIGHT - 60),
            r: 8 + Math.random() * 4,
            value: this.definition.collectibleValue
        };
    }

    createHazard() {
        const speed = this.definition.hazardSpeed;
        const angle = Math.random() * Math.PI * 2;
        return {
            x: 20 + Math.random() * (CONFIG.CANVAS_WIDTH - 40),
            y: 20 + Math.random() * (CONFIG.CANVAS_HEIGHT - 40),
            r: 10 + Math.random() * 8,
            vx: Math.cos(angle) * speed,
            vy: Math.sin(angle) * speed
        };
    }

    update(deltaTime) {
        if (this.isGameOver) {
            return;
        }

        this.timeRemaining -= deltaTime;
        if (this.timeRemaining <= 0) {
            this.endGame("Time up");
            return;
        }

        const input = this.engine.getInputManager();
        let dx = 0;
        let dy = 0;

        if (input.isKeyDown("ArrowLeft") || input.isKeyDown("KeyA")) dx -= 1;
        if (input.isKeyDown("ArrowRight") || input.isKeyDown("KeyD")) dx += 1;
        if (input.isKeyDown("ArrowUp") || input.isKeyDown("KeyW")) dy -= 1;
        if (input.isKeyDown("ArrowDown") || input.isKeyDown("KeyS")) dy += 1;

        if (dx !== 0 || dy !== 0) {
            const magnitude = Math.sqrt((dx * dx) + (dy * dy));
            this.player.x += (dx / magnitude) * this.player.speed * deltaTime;
            this.player.y += (dy / magnitude) * this.player.speed * deltaTime;
        }

        this.player.x = Math.max(this.player.r, Math.min(CONFIG.CANVAS_WIDTH - this.player.r, this.player.x));
        this.player.y = Math.max(this.player.r, Math.min(CONFIG.CANVAS_HEIGHT - this.player.r, this.player.y));

        for (const hazard of this.hazards) {
            hazard.x += hazard.vx * deltaTime;
            hazard.y += hazard.vy * deltaTime;

            if (hazard.x <= hazard.r || hazard.x >= CONFIG.CANVAS_WIDTH - hazard.r) {
                hazard.vx *= -1;
            }
            if (hazard.y <= hazard.r || hazard.y >= CONFIG.CANVAS_HEIGHT - hazard.r) {
                hazard.vy *= -1;
            }

            if (this.overlaps(this.player, hazard)) {
                this.endGame("Hit by hazard");
                return;
            }
        }

        const scoreManager = this.engine.getScoreManager();
        for (let i = 0; i < this.collectibles.length; i += 1) {
            const collectible = this.collectibles[i];
            if (this.overlaps(this.player, collectible)) {
                if (scoreManager) {
                    scoreManager.addScore(collectible.value);
                }
                this.collectibles[i] = this.createCollectible();
            }
        }
    }

    overlaps(a, b) {
        const dx = a.x - b.x;
        const dy = a.y - b.y;
        const distance = Math.sqrt((dx * dx) + (dy * dy));
        return distance <= (a.r + b.r);
    }

    endGame(reason) {
        this.isGameOver = true;
        this.gameOverReason = reason;
        if (typeof this.onGameOver === "function") {
            this.onGameOver(reason);
        }
    }

    draw(ctx) {
        ctx.clearRect(0, 0, CONFIG.CANVAS_WIDTH, CONFIG.CANVAS_HEIGHT);
        ctx.fillStyle = this.definition.background;
        ctx.fillRect(0, 0, CONFIG.CANVAS_WIDTH, CONFIG.CANVAS_HEIGHT);

        for (const collectible of this.collectibles) {
            ctx.fillStyle = "#f5d142";
            ctx.beginPath();
            ctx.arc(collectible.x, collectible.y, collectible.r, 0, Math.PI * 2);
            ctx.fill();
        }

        for (const hazard of this.hazards) {
            ctx.fillStyle = "#ff5c7a";
            ctx.beginPath();
            ctx.arc(hazard.x, hazard.y, hazard.r, 0, Math.PI * 2);
            ctx.fill();
        }

        ctx.fillStyle = "#4dd8ff";
        ctx.beginPath();
        ctx.arc(this.player.x, this.player.y, this.player.r, 0, Math.PI * 2);
        ctx.fill();

        ctx.fillStyle = "#ffffff";
        ctx.font = "16px sans-serif";
        ctx.fillText(`Time: ${Math.max(0, Math.ceil(this.timeRemaining))}s`, 10, 24);
        ctx.fillText(this.definition.tagline, 10, 46);
    }

    cleanup() {
        this.collectibles = [];
        this.hazards = [];
    }
}

const GAME_DEFINITIONS = [
    { id: "snake", name: "Snake Sprint", description: "Collect stars, dodge danger.", duration: 45, collectibleCount: 4, hazardCount: 2, collectibleValue: 10, playerSpeed: 230, hazardSpeed: 90, background: "#1f1b2e", tagline: "Classic speed and survival" },
    { id: "tetris", name: "Block Drift", description: "Fast lanes, tighter turns.", duration: 50, collectibleCount: 5, hazardCount: 3, collectibleValue: 8, playerSpeed: 220, hazardSpeed: 95, background: "#1b263b", tagline: "Precision movement challenge" },
    { id: "pong", name: "Pong Dash", description: "Arcade rally in motion.", duration: 40, collectibleCount: 3, hazardCount: 3, collectibleValue: 12, playerSpeed: 250, hazardSpeed: 110, background: "#16213e", tagline: "High speed, high score" },
    { id: "breakout", name: "Brick Break Run", description: "Avoid bounce zones.", duration: 55, collectibleCount: 6, hazardCount: 4, collectibleValue: 7, playerSpeed: 215, hazardSpeed: 100, background: "#2b2d42", tagline: "Steady pace, smart routing" },
    { id: "memory", name: "Memory Flux", description: "Calm mode with pressure.", duration: 60, collectibleCount: 7, hazardCount: 2, collectibleValue: 6, playerSpeed: 205, hazardSpeed: 85, background: "#2a1a3f", tagline: "Long run consistency" },
    { id: "game2048", name: "2048 Orbit", description: "Compact field, quick reflexes.", duration: 45, collectibleCount: 4, hazardCount: 5, collectibleValue: 9, playerSpeed: 240, hazardSpeed: 120, background: "#2d1e2f", tagline: "Dense hazard pattern" },
    { id: "asteroids", name: "Asteroid Weave", description: "Asteroid-like swarm play.", duration: 50, collectibleCount: 5, hazardCount: 6, collectibleValue: 8, playerSpeed: 245, hazardSpeed: 125, background: "#101820", tagline: "Survive the swarm" },
    { id: "tictactoe", name: "Grid Runner", description: "Simple look, tricky movement.", duration: 55, collectibleCount: 6, hazardCount: 3, collectibleValue: 7, playerSpeed: 215, hazardSpeed: 92, background: "#1d3557", tagline: "Control and timing" },
    { id: "whack", name: "Whack Rush", description: "Burst scoring mode.", duration: 35, collectibleCount: 8, hazardCount: 4, collectibleValue: 10, playerSpeed: 255, hazardSpeed: 115, background: "#2f3e46", tagline: "Quick bursts, big points" },
    { id: "jump", name: "Jump Run", description: "Endless-feel runner challenge.", duration: 60, collectibleCount: 7, hazardCount: 5, collectibleValue: 6, playerSpeed: 235, hazardSpeed: 105, background: "#14213d", tagline: "Last as long as possible" }
];
