--[[
adds attached file urls to the message for platforms that don't upload files
themselves
--]]

on("deliver", function(ev, dst)
	if dst:can("media") then
		return
	end
	for _, f in ipairs(ev.files) do
		if f.url ~= "" then
			ev:append(" " .. f.url)
		end
	end
end)
