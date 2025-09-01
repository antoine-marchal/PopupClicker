Add-Type -AssemblyName System.Windows.Forms

# Parameters
$message = "Do you want to continue?"
$title   = "Warning"
$buttons = [System.Windows.Forms.MessageBoxButtons]::YesNo
$default = [System.Windows.Forms.MessageBoxDefaultButton]::Button2  # Button2 => "No" for Yes/No

# Show the MessageBox (modal) and capture the result
$result = [System.Windows.Forms.MessageBox]::Show($message, $title, $buttons, [System.Windows.Forms.MessageBoxIcon]::Question, $default)

# Echo which button was clicked
if ($result -eq [System.Windows.Forms.DialogResult]::Yes) {
    Write-Host "You clicked: Yes"
} elseif ($result -eq [System.Windows.Forms.DialogResult]::No) {
    Write-Host "You clicked: No"
} else {
    Write-Host "Dialog closed with result: $result"
}
