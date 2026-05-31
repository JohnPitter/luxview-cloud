export namespace main {
	
	export class GameCard {
	    app_id: string;
	    name: string;
	    game: string;
	    display_name: string;
	    description: string;
	    enabled: boolean;
	    download_url: string;
	    server_ip: string;
	    auth_host: string;
	    installed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GameCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.app_id = source["app_id"];
	        this.name = source["name"];
	        this.game = source["game"];
	        this.display_name = source["display_name"];
	        this.description = source["description"];
	        this.enabled = source["enabled"];
	        this.download_url = source["download_url"];
	        this.server_ip = source["server_ip"];
	        this.auth_host = source["auth_host"];
	        this.installed = source["installed"];
	    }
	}
	export class GameSettings {
	    screen_width: number;
	    screen_height: number;
	    fullscreen: boolean;
	    mouse_sensitivity: number;
	    invert_mouse: boolean;
	    mouse_accel: boolean;
	    sound_volume: number;
	    music_volume: number;
	    gamma: number;
	
	    static createFrom(source: any = {}) {
	        return new GameSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.screen_width = source["screen_width"];
	        this.screen_height = source["screen_height"];
	        this.fullscreen = source["fullscreen"];
	        this.mouse_sensitivity = source["mouse_sensitivity"];
	        this.invert_mouse = source["invert_mouse"];
	        this.mouse_accel = source["mouse_accel"];
	        this.sound_volume = source["sound_volume"];
	        this.music_volume = source["music_volume"];
	        this.gamma = source["gamma"];
	    }
	}
	export class UpdateInfo {
	    available: boolean;
	    current: string;
	    version: string;
	    url: string;
	    notes: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.current = source["current"];
	        this.version = source["version"];
	        this.url = source["url"];
	        this.notes = source["notes"];
	    }
	}

}

