# duckhist zsh integration
duckhist_add_history() {
    duckhist add --tty "$TTY" -- "$1"
}
zshaddhistory_functions+=("duckhist_add_history")


function duckhist-history-selection() {
    BUFFER=`duckhist search`
    CURSOR=$#BUFFER
    zle reset-prompt
}

zle -N duckhist-history-selection
bindkey '^R' duckhist-history-selection
