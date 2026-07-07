# Render: vhs watch.tape  -> writes watch-frames.png/ (a dir of frames)
# Final still = the highest-numbered frame-text-*.png in that dir.

Output "watch-frames.png"

Set FontSize 18
Set Width 1360
Set Height 835
Set Padding 25
Set Theme "Builtin Dark"

Type "memhogs --watch --interval 2s"
Sleep 800ms
Enter
Sleep 3s
