package main

import (
	"fmt"
	"log"
	"math"
	"math/cmplx"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
	"github.com/gordonklaus/portaudio"
)

// Define styles
var titleStyle = lg.NewStyle().Bold(true).Background(lg.Color("#be8682")).Foreground(lg.Color("#f9ffff")).Padding(1, 2).MarginBottom(2)
var frequencyStyle = lg.NewStyle().Bold(true).Background(lg.Color("#5a3346")).Foreground(lg.Color("#a2c9b6")).MarginTop(1).Padding(0, 1)
var footerStyle = lg.NewStyle().Bold(true).MarginBottom(1).Padding(0, 1)
var footerHighlight = lg.NewStyle().Bold(true).Foreground(lg.Color("#be8682"))
var linesStyle = lg.NewStyle().Align(lg.Center)
var lineStyle = lg.NewStyle().Background(lg.Color("#d4f1f2")).Foreground(lg.Color("#2e1010")).Align(lg.Center).Bold(true).Width(3)
var highlightStyles = []lg.Style{
  lg.NewStyle().Background(lg.Color("#701a1a")).Foreground(lg.Color("#cea0c8")).Align(lg.Center).Width(3).Bold(true),
  lg.NewStyle().Background(lg.Color("#b07b69")).Foreground(lg.Color("#004455")).Align(lg.Center).Width(3).Bold(true),
  lg.NewStyle().Background(lg.Color("#c4c864")).Foreground(lg.Color("#fffefe")).Align(lg.Center).Width(3).Bold(true),
  lg.NewStyle().Background(lg.Color("#2e5a5c")).Foreground(lg.Color("#cea0c8")).Align(lg.Center).Width(3).Bold(true),
  lg.NewStyle().Background(lg.Color("#0f5c47")).Foreground(lg.Color("#eda0b5")).Align(lg.Center).Width(3).Bold(true),
}

// Define a tea.cmd to send dominantFrequency to bubbletea program
type DominantFrequencyMsg struct {
    Frequency float64
}

func DominantFrequencyCmd(frequency float64) tea.Cmd {
  return func() tea.Msg {
    return DominantFrequencyMsg{Frequency: frequency}
  }
}

// Defina main model
type MainModel struct {
  dominantFrequency float64
  w int
  h int
}

func InitialMainModel(stream *portaudio.Stream) MainModel {
  m := MainModel{}
  return m
} 

func (m MainModel) Init() tea.Cmd {
  return tea.WindowSize()
}

func (m MainModel) View() string {
  leftNote, centerNote, rightnote, rateIndex := HandleFrequency(m.dominantFrequency)

  s := ""
  s += titleStyle.Render("Lotus Tuner")
  s += "\n"
  s += footerStyle.Render(fmt.Sprintf(`"I'm beggining to see %s`, footerHighlight.Render(`the light!"`)))
  s += "\n"

  j := ""
  for i := 0; i < 9; i++ {
    text := "   "
    if i == 0 {
      text = leftNote
    }
    if i == 4 {
      text = centerNote
    }
    if i == 8 {
      text = rightnote
    }

    if i == rateIndex {
      // Repeat after 4th index
      if i > 4 {
        j += highlightStyles[8 - i].Render(text)
      } else {
        j += highlightStyles[i].Render(text)
      }
    } else {
      j += lineStyle.Render(text)
    }
    if i != 8 {
      j += " "
    }
  }
  s += linesStyle.Render(j)
  s += "\n"
  s += frequencyStyle.Render(fmt.Sprintf("[%.2f]", m.dominantFrequency))
  s += "\n"


	s = lg.Place(m.w, m.h, lg.Center, lg.Center, s)
  return s 
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
  switch msg := msg.(type) {
  case tea.KeyMsg:
    switch msg.String() {
    case "ctrl+c", "q":
      return m, tea.Quit
    }
  
  // Update dominantFrequency
  case DominantFrequencyMsg:
    m.dominantFrequency = msg.Frequency

  // Get terminal width and height
  case tea.WindowSizeMsg:
    m.w = msg.Width
    m.h = msg.Height
  }

  return m, nil
}


var (
  messageChannel = make(chan tea.Msg)
  quitChannel    = make(chan struct{})
)
func main() { 
  InitializeNotes()

  stream := CreateStream()
  err := stream.Start()
  if err != nil {
    log.Fatalf("Error starting stream: %v", err)
    stream.Close()
    portaudio.Terminate()
    os.Exit(1)
  }
  defer stream.Close()
  defer portaudio.Terminate()
  p := tea.NewProgram(InitialMainModel(stream), tea.WithAltScreen())

  go func() {
    if _, err := p.Run(); err != nil {
      log.Fatalf("There is been an error: %v", err)
      stream.Close()
      portaudio.Terminate()
      quitChannel <- struct{}{} // Signal that Bubbletea has quit
      os.Exit(1)
    }
    quitChannel <- struct{}{}
  }()

  // Check messageChannel and send DominantFrequencyMsg to the bubbletea program
  for {
    select {
      case msg := <-messageChannel:
        p.Send(msg)
      case <- quitChannel:
        return
      }
  }
}

// For better accuracy the ratio (sampleRate / framesPerBuffer) should be low (maybe?)
var (
  sampleRate = 2000
  framesPerBuffer = 2048 
  inputChannels = 1
)
func CreateStream() *portaudio.Stream {
  // Initialize PortAudio
  err := portaudio.Initialize()
  if err != nil {
    log.Fatal(err)
  }

  // Start stream and pass callback function
  stream, err := portaudio.OpenDefaultStream(inputChannels, 0, float64(sampleRate), framesPerBuffer, func(in []float32, timeInfo portaudio.StreamCallbackTimeInfo) {
    // Convert []float32 samples to []complex128 samples.
    complexSamples := make([]complex128, len(in))
    for i := range in {
      complexSamples[i] = complex(float64(in[i]), 0)
    } 

    // Calculate dominantFrequency and dominantNote
    dominantFrequency := CalculateDominantFrequencyBin(complexSamples, float64(sampleRate), float64(framesPerBuffer)) 

    // Send dominantFrequencyMsg to the bubbletea program
    messageChannel <- DominantFrequencyMsg{Frequency: dominantFrequency}
  })

  if err != nil {
    log.Fatalf("Error opening stream: %v", err)
  }

  return stream
}

func HandleFrequency(dominantFrequency float64) (string, string, string, int) { // left, center, right, rateIndex
  if dominantFrequency == 0 {
    return "","","",0
  }

  var leftNoteIndex, rightNoteIndex, centerNoteIndex = 0,0,0

  // Find closest note to the dominantFrequency
  closestIndex := 0 
  for i, note := range Notes {
    if math.Abs(dominantFrequency - note.Frequency) < math.Abs(dominantFrequency - Notes[closestIndex].Frequency) {
      closestIndex = i
    }
  }
  centerNoteIndex = closestIndex
  leftNoteIndex = centerNoteIndex - 1
  rightNoteIndex = centerNoteIndex + 1

  // Scale the distance in proportion and set the rateIndex
  totalDistance := Notes[rightNoteIndex].Frequency - Notes[leftNoteIndex].Frequency 
  distanceToLeft := dominantFrequency - (Notes[centerNoteIndex].Frequency - (Notes[centerNoteIndex].Frequency - Notes[leftNoteIndex].Frequency)/2)
  rateIndex := ((distanceToLeft) / (totalDistance/2)) * 9
  rateIndex = math.Floor(rateIndex)

  return Notes[leftNoteIndex].Name, Notes[centerNoteIndex].Name, Notes[rightNoteIndex].Name, int(rateIndex)
}

func CalculateDominantFrequencyBin(samples []complex128, sampleRate,windowSize float64) float64 {
  // Perform FFT on the samples.
  transformed := FFT(samples)

  // Find the dominant frequency
  maxMag := 0.0
  maxIndex := 0
  for i, cmplxValue := range transformed[:len(transformed)/2.0] {
    magnitude := math.Sqrt(real(cmplxValue)*real(cmplxValue) + imag(cmplxValue)*imag(cmplxValue))
    if magnitude > maxMag {
      maxMag = magnitude
      maxIndex = i
    }
  }

  // Calculate the frequency of the dominant component
  return float64(maxIndex) * sampleRate / windowSize 
}

func FFT(samples []complex128) []complex128 {
  // Find the number of samples we have.
  n := len(samples)
  
  // End of recursive once we have only 1 sample.
  if n == 1 {
    return samples
  }
  
  // Split the samples into even and odd subsums.
  // Find half the total number of samples.
  m := n / 2.0

  // Declare an even and odd []complex128
  Xeven := make([]complex128, m)
  Xodd := make([]complex128, m)

  // Input the even and odd samples
  for i := 0; i < m; i++ {
    Xeven[i] = samples[2*i + 1]
    Xodd[i] = samples[2*i]
  }

  // Perform the recursive FFT operation on the even and odd samples.
  Feven, Fodd := make([]complex128, m), make([]complex128, m)
  Feven, Fodd = FFT(Xeven), FFT(Xodd)

  /* END OF RECURSION */
  
  // Declare frequency bins
  freqbins := make([]complex128, n)
  
  // Combine the values found
  for k := 0; k != n/2; k++ {
    // For each split set, we need to multiply a k-dependent complex number
    // by the odd subsum.
    complexExponential := cmplx.Rect(1, -2.0 * math.Pi * float64(k) / float64(n)) * Fodd[k]
    freqbins[k] = Feven[k] + complexExponential
    
    // Everytime you add pi, exponential changes sign.
    freqbins[k + (n / 2)] = Feven[k] - complexExponential
  }

  return freqbins
}
