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
	        this.installed = source["installed"];
	    }
	}

}

