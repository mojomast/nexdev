class ScoreManager {
    constructor(gameId) {
        this.gameId = gameId;
        this.score = 0;
        this.highScore = this.loadHighScore();
    }

    addScore(points) {
        this.score += points;
    }

    setScore(score) {
        this.score = score;
    }

    getScore() {
        return this.score;
    }

    getHighScore() {
        return this.highScore;
    }

    saveHighScore() {
        if (this.score > this.highScore) {
            this.highScore = this.score;
            localStorage.setItem(this.getStorageKey(), this.highScore.toString());
            return true;
        }
        return false;
    }

    loadHighScore() {
        const stored = localStorage.getItem(this.getStorageKey());
        return stored ? parseInt(stored, 10) : 0;
    }

    resetScore() {
        this.score = 0;
    }

    getStorageKey() {
        return CONFIG.HIGH_SCORE_KEY_PREFIX + this.gameId;
    }
}
