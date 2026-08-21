-- /addmoney 1000000
-- /addmoney Owner, 1000000

local addMoney = TalkAction("/addmoney")

function addMoney.onSay(player, words, param)
	logCommand(player, words, param)

	if param == "" then
		player:sendCancelMessage("Informe um valor ou use /addmoney Nome, valor.")
		return true
	end

	local split = param:split(",")
	local name = player:getName()
	local amount

	if split[2] then
		name = split[1]:trim()
		amount = tonumber(split[2])
	else
		amount = tonumber(split[1])
	end

	if not amount or amount <= 0 then
		player:sendCancelMessage("Valor inválido.")
		return true
	end

	local normalizedName = Game.getNormalizedPlayerName(name)
	if not normalizedName then
		player:sendCancelMessage("O personagem " .. name .. " não existe.")
		return true
	end

	local targetPlayer = Player(normalizedName)
	if targetPlayer then
		if not targetPlayer:addMoney(amount) then
			player:sendCancelMessage("Não foi possível colocar o dinheiro na mochila.")
			return true
		end
		player:sendTextMessage(MESSAGE_EVENT_ADVANCE, "Adicionado " .. amount .. " gold coins à mochila de " .. normalizedName .. ".")
		targetPlayer:sendTextMessage(MESSAGE_EVENT_ADVANCE, player:getName() .. " adicionou " .. amount .. " gold coins à sua mochila.")
	elseif not Bank.credit(normalizedName, amount) then
		player:sendCancelMessage("Não foi possível adicionar o dinheiro ao banco.")
		return true
	else
		player:sendTextMessage(MESSAGE_EVENT_ADVANCE, "Adicionado " .. amount .. " gold coins ao banco de " .. normalizedName .. ".")
	end
	return true
end

addMoney:separator(" ")
addMoney:groupType("god")
addMoney:register()
