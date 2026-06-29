class InputManager {
    constructor() {
        this.keys = {};
        this.mouse = { x: 0, y: 0, pressed: false };
        this.touch = { active: false, x: 0, y: 0 };
        this.canvas = null;
    }

    init(canvas) {
        this.canvas = canvas;
        
        window.addEventListener('keydown', (e) => {
            this.keys[e.code] = true;
            e.preventDefault();
        });

        window.addEventListener('keyup', (e) => {
            this.keys[e.code] = false;
        });

        canvas.addEventListener('mousedown', (e) => {
            this.mouse.pressed = true;
            this.mouse.x = e.offsetX;
            this.mouse.y = e.offsetY;
        });

        canvas.addEventListener('mouseup', (e) => {
            this.mouse.pressed = false;
            this.mouse.x = e.offsetX;
            this.mouse.y = e.offsetY;
        });

        canvas.addEventListener('mousemove', (e) => {
            this.mouse.x = e.offsetX;
            this.mouse.y = e.offsetY;
        });

        canvas.addEventListener('touchstart', (e) => {
            e.preventDefault();
            this.touch.active = true;
            const rect = canvas.getBoundingClientRect();
            this.touch.x = e.touches[0].clientX - rect.left;
            this.touch.y = e.touches[0].clientY - rect.top;
        });

        canvas.addEventListener('touchend', (e) => {
            e.preventDefault();
            this.touch.active = false;
        });

        canvas.addEventListener('touchmove', (e) => {
            e.preventDefault();
            const rect = canvas.getBoundingClientRect();
            this.touch.x = e.touches[0].clientX - rect.left;
            this.touch.y = e.touches[0].clientY - rect.top;
        });
    }

    isKeyDown(keyCode) {
        return this.keys[keyCode] || false;
    }

    getMousePosition() {
        return { x: this.mouse.x, y: this.mouse.y };
    }

    isMousePressed() {
        return this.mouse.pressed;
    }

    getTouchPosition() {
        return { x: this.touch.x, y: this.touch.y, active: this.touch.active };
    }

    reset() {
        this.keys = {};
        this.mouse = { x: 0, y: 0, pressed: false };
        this.touch = { active: false, x: 0, y: 0 };
    }
}
