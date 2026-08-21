local ghost = TalkAction("/ghost")

function ghost.onSay(player, words, param)
	logCommand(player, words, param)

	local position = player:getPosition()
	local isGhost = not player:isInGhostMode()
	player:setGhostMode(isGhost)

	if isGhost then
		player:sendTextMessage(MESSAGE_HOTKEY_PRESSED, "Você está invisível.")
		position:sendMagicEffect(CONST_ME_YALAHARIGHOST)
	else
		player:sendTextMessage(MESSAGE_HOTKEY_PRESSED, "Você está visível novamente.")
		position.x = position.x + 1
		position:sendMagicEffect(CONST_ME_SMOKE)
	end
	return true
end

ghost:separator(" ")
ghost:groupType("god")
ghost:register()
