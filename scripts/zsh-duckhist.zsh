# duckhist zsh integration
duckhist_add_history() {
    duckhist add -- "$1"
}
zshaddhistory_functions+=("duckhist_add_history")
