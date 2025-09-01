Add-Type -AssemblyName System.Windows.Forms

# Parameters
$message = "This is a test popup."
$title   = "Warning"
$buttons = [System.Windows.Forms.MessageBoxButtons]::OK

# Show the MessageBox (modal)
[System.Windows.Forms.MessageBox]::Show($message, $title, $buttons) | Out-Null
