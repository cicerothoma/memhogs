# Render: vhs compact.tape  -> writes compact-frames.png/ (a dir of frames)
# Final still = the highest-numbered frame-text-*.png in that dir.

Output "compact-frames.png"

Set FontSize 18
Set Width 1360
Set Height 835
Set Padding 25
Set Theme "Builtin Dark"

Type "memhogs --compact --top 30"
Sleep 800ms
Enter
Sleep 3s
