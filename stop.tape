# Renders docs/stop.gif (the "how to stop a hog" demo). Regenerate with:
#   vhs stop.tape   (needs memhogs on PATH)
Output docs/stop.gif

Set FontSize 17
Set Width 1270
Set Height 520
Set Padding 18
Set TypingSpeed 55ms
Set Theme "Builtin Dark"

Type "memhogs --stop-hint --compact --top 4"
Sleep 600ms
Enter
Sleep 4s

Type "memhogs stop warp"
Sleep 400ms
Enter
Sleep 4.5s
