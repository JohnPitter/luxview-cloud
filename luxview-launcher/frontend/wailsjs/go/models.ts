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
	    display_mode: string;
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
	        this.display_mode = source["display_mode"];
	        this.mouse_sensitivity = source["mouse_sensitivity"];
	        this.invert_mouse = source["invert_mouse"];
	        this.mouse_accel = source["mouse_accel"];
	        this.sound_volume = source["sound_volume"];
	        this.music_volume = source["music_volume"];
	        this.gamma = source["gamma"];
	    }
	}
	export class Metin2Settings {
	    screen_width: number;
	    screen_height: number;
	    bpp: number;
	    frequency: number;
	    windowed: boolean;
	    software_cursor: boolean;
	    object_culling: boolean;
	    visibility: number;
	    music_volume: number;
	    sound_volume: number;
	    gamma: number;
	    pre_loading_delay: number;
	    decompressed_texture: boolean;
	    always_view_name: boolean;
	    show_refine_dialog: boolean;
	    fog_mode: boolean;
	    night_mode: boolean;
	    snow_mode: boolean;
	    snow_texture: boolean;
	    show_mob_level: boolean;
	    show_mob_ai_flag: boolean;
	    auto_pickup: boolean;
	    extended_fov: boolean;
	    effect_level: number;
	    private_shop_level: number;
	    drop_item_level: number;
	    pet_status: boolean;
	    npc_name_status: boolean;
	    show_dice_info: boolean;
	    poly_dog_mode: boolean;
	    premium_affect: boolean;
	    time_system: boolean;
	    enb_mode_status: boolean;
	    use_default_ime: boolean;
	    software_tiling: number;
	    shadow_level: number;
	
	    static createFrom(source: any = {}) {
	        return new Metin2Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.screen_width = source["screen_width"];
	        this.screen_height = source["screen_height"];
	        this.bpp = source["bpp"];
	        this.frequency = source["frequency"];
	        this.windowed = source["windowed"];
	        this.software_cursor = source["software_cursor"];
	        this.object_culling = source["object_culling"];
	        this.visibility = source["visibility"];
	        this.music_volume = source["music_volume"];
	        this.sound_volume = source["sound_volume"];
	        this.gamma = source["gamma"];
	        this.pre_loading_delay = source["pre_loading_delay"];
	        this.decompressed_texture = source["decompressed_texture"];
	        this.always_view_name = source["always_view_name"];
	        this.show_refine_dialog = source["show_refine_dialog"];
	        this.fog_mode = source["fog_mode"];
	        this.night_mode = source["night_mode"];
	        this.snow_mode = source["snow_mode"];
	        this.snow_texture = source["snow_texture"];
	        this.show_mob_level = source["show_mob_level"];
	        this.show_mob_ai_flag = source["show_mob_ai_flag"];
	        this.auto_pickup = source["auto_pickup"];
	        this.extended_fov = source["extended_fov"];
	        this.effect_level = source["effect_level"];
	        this.private_shop_level = source["private_shop_level"];
	        this.drop_item_level = source["drop_item_level"];
	        this.pet_status = source["pet_status"];
	        this.npc_name_status = source["npc_name_status"];
	        this.show_dice_info = source["show_dice_info"];
	        this.poly_dog_mode = source["poly_dog_mode"];
	        this.premium_affect = source["premium_affect"];
	        this.time_system = source["time_system"];
	        this.enb_mode_status = source["enb_mode_status"];
	        this.use_default_ime = source["use_default_ime"];
	        this.software_tiling = source["software_tiling"];
	        this.shadow_level = source["shadow_level"];
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

