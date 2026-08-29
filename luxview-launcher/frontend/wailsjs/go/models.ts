export namespace main {
	
	export class CommunityGamePlayers {
	    app_id: string;
	    game: string;
	    name: string;
	    display_name: string;
	    players: number;
	    max_players: number;
	
	    static createFrom(source: any = {}) {
	        return new CommunityGamePlayers(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.app_id = source["app_id"];
	        this.game = source["game"];
	        this.name = source["name"];
	        this.display_name = source["display_name"];
	        this.players = source["players"];
	        this.max_players = source["max_players"];
	    }
	}
	export class CommunityMessage {
	    id: string;
	    username: string;
	    body: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new CommunityMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.username = source["username"];
	        this.body = source["body"];
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CommunityPost {
	    id: string;
	    app_id: string;
	    game: string;
	    game_name: string;
	    display_name: string;
	    title: string;
	    body: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new CommunityPost(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.app_id = source["app_id"];
	        this.game = source["game"];
	        this.game_name = source["game_name"];
	        this.display_name = source["display_name"];
	        this.title = source["title"];
	        this.body = source["body"];
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CommunitySnapshot {
	    players_online: number;
	    chat_online: number;
	    games: CommunityGamePlayers[];
	    posts: CommunityPost[];
	    chat: CommunityMessage[];
	
	    static createFrom(source: any = {}) {
	        return new CommunitySnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.players_online = source["players_online"];
	        this.chat_online = source["chat_online"];
	        this.games = this.convertValues(source["games"], CommunityGamePlayers);
	        this.posts = this.convertValues(source["posts"], CommunityPost);
	        this.chat = this.convertValues(source["chat"], CommunityMessage);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GameCard {
	    app_id: string;
	    name: string;
	    game: string;
	    display_name: string;
	    description: string;
	    enabled: boolean;
	    download_url: string;
	    patch_url: string;
	    base_url: string;
	    server_ip: string;
	    auth_host: string;
	    client_hash: string;
	    base_hash: string;
	    installed: boolean;
	    update_available: boolean;
	
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
	        this.patch_url = source["patch_url"];
	        this.base_url = source["base_url"];
	        this.server_ip = source["server_ip"];
	        this.auth_host = source["auth_host"];
	        this.client_hash = source["client_hash"];
	        this.base_hash = source["base_hash"];
	        this.installed = source["installed"];
	        this.update_available = source["update_available"];
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
	export class MuServerInfo {
	    id: number;
	    name: string;
	    load: number;
	
	    static createFrom(source: any = {}) {
	        return new MuServerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.load = source["load"];
	    }
	}
	export class PlayerSession {
	    token: string;
	    username: string;
	    cash_points: number;
	    reward_points: number;
	
	    static createFrom(source: any = {}) {
	        return new PlayerSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.username = source["username"];
	        this.cash_points = source["cash_points"];
	        this.reward_points = source["reward_points"];
	    }
	}
	export class ShopItem {
	    id: string;
	    name: string;
	    description: string;
	    template_id: string;
	    currency: string;
	    price: number;
	    grant: string;
	    amount: number;
	
	    static createFrom(source: any = {}) {
	        return new ShopItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.template_id = source["template_id"];
	        this.currency = source["currency"];
	        this.price = source["price"];
	        this.grant = source["grant"];
	        this.amount = source["amount"];
	    }
	}
	export class ShopBuyResult {
	    item: ShopItem;
	    cash_points: number;
	    reward_points: number;
	
	    static createFrom(source: any = {}) {
	        return new ShopBuyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.item = this.convertValues(source["item"], ShopItem);
	        this.cash_points = source["cash_points"];
	        this.reward_points = source["reward_points"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class UpdateInfo {
	    available: boolean;
	    current: string;
	    version: string;
	    url: string;
	    sha256: string;
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
	        this.sha256 = source["sha256"];
	        this.notes = source["notes"];
	    }
	}

}

