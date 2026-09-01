package service

import (
	"strconv"
	"strings"
)

// Nomes oficiais (wiki / vocations.xml / client) — usados só para exibir o roster.

func tibiaVocationName(id int) string {
	switch id {
	case 1:
		return "Sorcerer"
	case 2:
		return "Druid"
	case 3:
		return "Paladin"
	case 4:
		return "Knight"
	case 5:
		return "Master Sorcerer"
	case 6:
		return "Elder Druid"
	case 7:
		return "Royal Paladin"
	case 8:
		return "Elite Knight"
	case 9:
		return "Monk"
	case 10:
		return "Exalted Monk"
	default:
		if id == 0 {
			return "Sem vocação"
		}
		return "Vocação " + strconv.Itoa(id)
	}
}

func tibiaTownName(id int) string {
	// OTServBR-Global / Canary (wiki OpenTibiaBR).
	switch id {
	case 1:
		return "Dawnport Tutorial"
	case 2:
		return "Dawnport"
	case 3:
		return "Rookgaard"
	case 4:
		return "Island of Destiny"
	case 5:
		return "Ab'Dendriel"
	case 6:
		return "Carlin"
	case 7:
		return "Kazordoon"
	case 8:
		return "Thais"
	case 9:
		return "Venore"
	case 10:
		return "Ankrahmun"
	case 11:
		return "Edron"
	case 12:
		return "Farmine"
	case 13:
		return "Darashia"
	case 14:
		return "Liberty Bay"
	case 15:
		return "Port Hope"
	case 16:
		return "Svargrond"
	case 17:
		return "Yalahar"
	case 18:
		return "Gray Beach"
	case 19:
		return "Krailos"
	case 20:
		return "Rathleton"
	case 21:
		return "Roshamuul"
	case 28:
		return "Issavi"
	default:
		if id == 0 {
			return "Desconhecida"
		}
		return "Cidade " + strconv.Itoa(id)
	}
}

func metinJobName(job int) string {
	// Race 0–7 clássico + 8 Lycan. job%4 = classe; 4+ = feminino.
	if job == 8 {
		return "Lycan"
	}
	sex := ""
	if job >= 4 {
		sex = " (F)"
		job -= 4
	} else {
		sex = " (M)"
	}
	switch job {
	case 0:
		return "Guerreiro" + sex
	case 1:
		return "Ninja" + sex
	case 2:
		return "Sura" + sex
	case 3:
		return "Shaman" + sex
	default:
		return "Classe " + strconv.Itoa(job)
	}
}

func metinMapName(id int) string {
	// map_index do ch1 clássico (wiki Metin2 / locale map).
	switch id {
	case 1:
		return "Chunjo — Vila"
	case 3:
		return "Jinno — Vila"
	case 21:
		return "Shinsoo — Vila"
	case 41:
		return "Vale de Seungryong"
	case 42:
		return "Montanha Sohan"
	case 43:
		return "Deserto Yongbi"
	case 44:
		return "Floresta do Fantasma"
	case 61:
		return "Montanha Sohan (neve)"
	case 62:
		return "Deserto Yongbi"
	case 63:
		return "Floresta do Fantasma"
	case 64:
		return "Vale de Seungryong"
	case 65:
		return "Templo Hwang"
	case 66:
		return "Caverna das Aranhas"
	case 67, 68, 69:
		return "Calabouço das Aranhas"
	case 70, 71, 72, 73:
		return "Gautama"
	case 104:
		return "Ilha da Presa Vermelha"
	case 107:
		return "Deserto Yongbi (campo)"
	case 110:
		return "Castelo da Guilda"
	case 301, 302, 303, 304:
		return "Guild War"
	default:
		if id == 0 {
			return "Limbo"
		}
		return "Mapa " + strconv.Itoa(id)
	}
}

func muClassName(id int) string {
	// Byte de classe Season 6 / 97D (wiki MU Online).
	switch id {
	case 0:
		return "Dark Wizard"
	case 1:
		return "Soul Master"
	case 2, 3:
		return "Grand Master"
	case 16:
		return "Dark Knight"
	case 17:
		return "Blade Knight"
	case 18, 19:
		return "Blade Master"
	case 32:
		return "Fairy Elf"
	case 33:
		return "Muse Elf"
	case 34, 35:
		return "High Elf"
	case 48:
		return "Magic Gladiator"
	case 49, 50:
		return "Duel Master"
	case 64:
		return "Dark Lord"
	case 65, 66:
		return "Lord Emperor"
	case 80:
		return "Summoner"
	case 81:
		return "Bloody Summoner"
	case 82, 83:
		return "Dimension Master"
	case 96:
		return "Rage Fighter"
	case 97, 98:
		return "Fist Master"
	default:
		return "Classe " + strconv.Itoa(id)
	}
}

func muMapName(id int) string {
	switch id {
	case 0:
		return "Lorencia"
	case 1:
		return "Dungeon"
	case 2:
		return "Devias"
	case 3:
		return "Noria"
	case 4:
		return "Lost Tower"
	case 6:
		return "Arena"
	case 7:
		return "Atlans"
	case 8:
		return "Tarkan"
	case 9:
		return "Devil Square"
	case 10:
		return "Icarus"
	case 11:
		return "Blood Castle 1"
	case 24:
		return "Kalima 1"
	case 30:
		return "Valley of Loren"
	case 31:
		return "Land of Trials"
	case 33:
		return "Aida"
	case 34:
		return "Crywolf"
	case 37:
		return "Kanturu Relics"
	case 38:
		return "Kanturu Remain"
	case 41:
		return "Barracks"
	case 42:
		return "Refuge"
	case 51:
		return "Elbeland"
	case 56:
		return "Swamp of Calmness"
	case 57:
		return "Raklion"
	case 63:
		return "Vulcanus"
	case 80:
		return "Kalrutan"
	case 81:
		return "Raklion Boss"
	default:
		return "Mapa " + strconv.Itoa(id)
	}
}

// formatOpenMUServer is the same label the launcher uses: season name + channel
// (ConnectServer packs channel as ServerID%20, displayed 1-based).
func formatOpenMUServer(serverID int, description string) string {
	desc := strings.TrimSpace(description)
	if serverID == 0 && desc == "" {
		return ""
	}
	channel := serverID%20 + 1
	return openMUServerDisplayName(desc, serverID/20) + " - " + strconv.Itoa(channel)
}

func openMUServerDisplayName(description string, group int) string {
	d := strings.ToLower(description)
	switch {
	case strings.Contains(d, "99d") || strings.Contains(d, "hard"):
		return "Season 99d"
	case strings.Contains(d, "s2") || strings.Contains(d, "season 2"):
		return "Season 2"
	case strings.Contains(d, "s6") || strings.Contains(d, "season 6"):
		return "Season 6"
	}
	switch group {
	case 0:
		return "Season 6"
	case 1:
		return "Season 99d"
	case 2:
		return "Season 2"
	}
	if description != "" {
		return description
	}
	return "Servidor " + strconv.Itoa(group+1)
}
