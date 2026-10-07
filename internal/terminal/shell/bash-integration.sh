# ttt bash integration, loaded with --rcfile in place of ~/.bashrc.
[ -r ~/.bashrc ] && . ~/.bashrc

__ttt_prompt() {
	local status=$?
	if [[ $PS1 != *'133;A'* ]]; then
		# readline repaints only the last line of a multi-line prompt.
		local head= tail=$PS1
		if [[ $tail == *'\n'* ]]; then
			head=${tail%'\n'*}'\n'
			tail=${tail##*'\n'}
		fi
		if [[ $tail == *$'\n'* ]]; then
			head+=${tail%$'\n'*}$'\n'
			tail=${tail##*$'\n'}
		fi
		PS1=$head'\[\e]133;A;redraw=last\a\]'$tail
	fi
	[[ ${PS0-} == *'133;C'* ]] || PS0='\e]133;C\a'"${PS0-}"
	return "$status"
}

[[ $PROMPT_COMMAND == *__ttt_prompt* ]] || PROMPT_COMMAND="${PROMPT_COMMAND:+$PROMPT_COMMAND
}__ttt_prompt"
