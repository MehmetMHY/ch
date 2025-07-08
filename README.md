# Cha-Go

A powerful Go implementation of a chat client with support for multiple AI platforms including OpenAI, Groq, DeepSeek, Anthropic, and XAI. Features web search integration, multiple interaction modes, and comprehensive command-line functionality.

## 🚀 Features

### Core Functionality

- **Multi-platform AI support** - Chat with models from OpenAI, Groq, DeepSeek, Anthropic, and XAI
- **Interactive chat interface** - Real-time streaming responses with color-coded output
- **Direct query mode** - Run single queries from command line
- **Chat history management** - Maintains conversation context throughout session
- **Export functionality** - Save chat history to uniquely named files

### Advanced Features

- **Web search integration** - Search the web using SearXNG with AI-generated responses and IEEE citations
- **Platform switching** - Seamlessly switch between AI platforms and models
- **Terminal input mode** - Multi-line input using your preferred editor
- **Reasoning model support** - Special handling for reasoning models (o1, o3, etc.) with loading animations
- **Fuzzy selection** - Uses `fzf` for interactive model and platform selection

### Command Interface

- **Interactive commands** - Special commands for various functions during chat
- **Command-line arguments** - Full CLI support for automation and scripting
- **Pipe support** - Works with shell pipes and redirects

## 📋 Prerequisites

- Go 1.21 or later
- Docker (for SearXNG web search feature)
- `fzf` command-line tool for interactive selection
- Python 3 and PyYAML (for SearXNG setup)

## 🛠 Installation

1. **Clone the repository**

   ```bash
   git clone <repository-url>
   cd cha-go
   ```

2. **Set up API keys** (choose platforms you want to use)

   ```bash
   export OPENAI_API_KEY="your-openai-key"        # For OpenAI
   export GROQ_API_KEY="your-groq-key"            # For Groq
   export DEEP_SEEK_API_KEY="your-deepseek-key"   # For DeepSeek
   export ANTHROPIC_API_KEY="your-anthropic-key"  # For Anthropic
   export XAI_API_KEY="your-xai-key"              # For XAI
   ```

3. **Install dependencies**

   ```bash
   go mod tidy
   ```

4. **Build the application**

   ```bash
   chmod +x build.sh
   ./build.sh
   ```

5. **Optional: Set up SearXNG for web search**
   ```bash
   cd sxng
   pip3 install -r requirements.txt
   python3 run.py
   ```

## 🎯 Usage

### Command Line Options

```bash
./cha-go [OPTIONS] [QUERY]

Options:
  -h              Show help
  -p [platform]   Switch platform (openai, groq, deepseek, anthropic, xai)
  -m [model]      Specify model to use

Examples:
  ./cha-go                                    # Interactive mode
  ./cha-go "explain quantum computing"        # Direct query
  ./cha-go -p groq "what is AI?"             # Use Groq platform
  ./cha-go -p openai -m gpt-4o "hello"       # Specify platform and model
  ./cha-go -h                                # Show help
```

### Interactive Commands

| Command         | Description                                             |
| --------------- | ------------------------------------------------------- |
| `!q`            | Exit the application                                    |
| `!h`            | Show help and available commands                        |
| `!c`            | Clear chat history                                      |
| `!sm`           | Interactive model selection (uses fzf)                  |
| `!sm <model>`   | Switch to specific model directly                       |
| `!p`            | Interactive platform selection                          |
| `!p <platform>` | Switch to specific platform                             |
| `!t`            | Terminal input mode (opens editor for multi-line input) |
| `!e`            | Export chat history to file                             |
| `!w <query>`    | Web search using SearXNG (requires SearXNG setup)       |

### Platform Support

| Platform      | Models                               | API Key Required    |
| ------------- | ------------------------------------ | ------------------- |
| **OpenAI**    | GPT-4, GPT-4o, GPT-3.5, o1, o3, etc. | `OPENAI_API_KEY`    |
| **Groq**      | Llama, Mixtral, Gemma models         | `GROQ_API_KEY`      |
| **DeepSeek**  | DeepSeek models                      | `DEEP_SEEK_API_KEY` |
| **Anthropic** | Claude models                        | `ANTHROPIC_API_KEY` |
| **XAI**       | Grok models                          | `XAI_API_KEY`       |

## 🔍 Web Search Feature

The web search feature uses SearXNG to provide AI responses with real web data and citations:

1. **Set up SearXNG** (one-time setup)

   ```bash
   cd sxng
   python3 run.py
   ```

2. **Use web search in chat**

   ```bash
   User: !w what is the latest news about AI?
   ```

3. **Features**
   - IEEE citation format in responses
   - Up to 8 search results per query
   - Automatic JSON format configuration
   - References section with URLs

## 💡 Example Sessions

### Basic Chat

```bash
$ ./cha-go
Chatting with OPENAI Model: gpt-4o-mini
User: Hello, how are you?
Hello! I'm doing well, thank you for asking. How can I help you today?

User: !sm
# fzf interface appears to select model

User: !p groq
# Switches to Groq platform and shows model selection
```

### Direct Query Mode

```bash
$ ./cha-go -p anthropic "explain machine learning"
# Gets response from Claude model without entering interactive mode

$ echo "what is 2+2?" | ./cha-go -p groq
# Uses pipe input with Groq platform
```

### Web Search

```bash
User: !w current weather in Tokyo
# Searches web for Tokyo weather and provides AI response with citations

The current weather in Tokyo shows partly cloudy conditions with temperatures
around 15°C (59°F) [1]. According to recent weather data, there's a 20% chance
of rain with light winds from the northeast [2].

References:
[1] Tokyo Weather - Japan Meteorological Agency, https://jma.go.jp/...
[2] Current Tokyo Conditions - Weather.com, https://weather.com/...
```

### Terminal Input Mode

```bash
User: !t
# Opens your preferred editor (default: hx) for multi-line input
# Write your message, save and close
> This is a longer message
> that spans multiple lines
> with complex formatting
[AI response appears here]
```

## ⚙️ Configuration

### Default Settings

- **Default platform**: OpenAI
- **Default model**: gpt-4o-mini
- **Preferred editor**: hx (for terminal input mode)
- **System prompt**: "You are a helpful assistant powered by a simple Go chat client. Be concise, clear, and accurate."

### Environment Variables

All API keys are loaded from environment variables. Set only the ones for platforms you plan to use.

### SearXNG Configuration

The SearXNG setup script automatically:

- Creates Docker configuration
- Enables JSON format for API access
- Handles first-time setup
- Provides error handling and recovery

## 🏗 Architecture

### Special Features

- **Reasoning Model Detection**: Automatically detects reasoning models (o1, o3) and provides loading animations instead of streaming
- **Terminal Detection**: Behaves differently when run in terminal vs piped input
- **Color-coded Output**: Uses ANSI colors for better readability
- **Error Handling**: Graceful error handling with informative messages
- **Session Management**: Maintains context and history throughout session

### File Structure

```
cha-go/
├── main.go              # Main application
├── go.mod              # Go dependencies
├── go.sum              # Go dependency checksums
├── build.sh            # Build script
├── README.md           # This file
└── sxng/               # SearXNG integration
    ├── run.py          # SearXNG setup script
    ├── requirements.txt # Python dependencies
    ├── README.md       # SearXNG documentation
    └── searxng/        # Configuration directory (created on first run)
```

## 🤝 Dependencies

### Go Dependencies

- `github.com/sashabaranov/go-openai` - OpenAI API client
- `github.com/google/uuid` - UUID generation for exports

### External Tools

- `fzf` - Fuzzy finder for interactive selection
- `docker` - For SearXNG web search functionality

### Optional Dependencies

- `python3` and `PyYAML` - For SearXNG setup automation

## 📝 Notes

- **Streaming Responses**: All platforms support real-time streaming except reasoning models
- **Cross-platform**: Works on Linux, macOS, and Windows
- **Memory Efficient**: Maintains conversation context without excessive memory usage
- **Error Recovery**: Handles network issues and API errors gracefully
- **Extensible**: Easy to add new AI platforms following the existing pattern

## 🚨 Troubleshooting

### Common Issues

1. **API Key not found**: Make sure environment variables are set correctly
2. **fzf not found**: Install fzf using your package manager
3. **SearXNG not working**: Ensure Docker is running and run `python3 run.py` in sxng directory
4. **Platform switching fails**: Verify API keys for target platform are set

### Web Search Issues

1. **JSON format errors**: Run the SearXNG setup script again
2. **Container conflicts**: Stop existing containers with `docker stop searxng-search`
3. **Permission issues**: Ensure Docker has proper permissions

For more detailed troubleshooting, check the logs or run with verbose flags.
