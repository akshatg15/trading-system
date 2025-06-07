#!/usr/bin/env python3
"""
MT5 Bridge Setup Script with UV
Fast and modern Python package management for the trading system
"""

import os
import sys
import subprocess
import platform
from pathlib import Path

def run_command(cmd, check=True, capture_output=False):
    """Run a command and handle errors"""
    try:
        if capture_output:
            result = subprocess.run(cmd, shell=True, check=check, 
                                  capture_output=True, text=True)
            return result.stdout.strip()
        else:
            subprocess.run(cmd, shell=True, check=check)
            return True
    except subprocess.CalledProcessError as e:
        print(f"❌ Command failed: {cmd}")
        print(f"Error: {e}")
        return False

def check_python_version():
    """Check if Python version is compatible"""
    version = sys.version_info
    if version.major != 3 or version.minor < 9:
        print(f"❌ Python 3.9+ required, found {version.major}.{version.minor}")
        return False
    print(f"✅ Python {version.major}.{version.minor}.{version.micro}")
    return True

def check_uv_installed():
    """Check if uv is installed"""
    try:
        version = run_command("uv --version", capture_output=True)
        if version:
            print(f"✅ uv is installed: {version}")
            return True
    except:
        pass
    
    print("❌ uv not found")
    return False

def install_uv():
    """Install uv package manager"""
    print("📦 Installing uv...")
    
    system = platform.system().lower()
    
    if system == "windows":
        # Windows installation
        cmd = 'powershell -c "irm https://astral.sh/uv/install.ps1 | iex"'
    else:
        # Unix/Linux/macOS installation  
        cmd = 'curl -LsSf https://astral.sh/uv/install.sh | sh'
    
    if run_command(cmd):
        print("✅ uv installed successfully")
        
        # Add to PATH for current session
        if system != "windows":
            os.environ["PATH"] = f"{os.path.expanduser('~/.cargo/bin')}:{os.environ['PATH']}"
        
        return True
    else:
        print("❌ Failed to install uv")
        return False

def setup_mt5_bridge():
    """Setup MT5 bridge with uv"""
    print("🔧 Setting up MT5 Bridge with uv...")
    
    # Get script directory and bridge directory
    script_dir = Path(__file__).parent
    bridge_dir = script_dir.parent / "mt5-bridge"
    
    if not bridge_dir.exists():
        print(f"❌ MT5 bridge directory not found: {bridge_dir}")
        return False
    
    # Change to bridge directory
    os.chdir(bridge_dir)
    print(f"📁 Working in: {bridge_dir}")
    
    # Install dependencies with uv
    print("📦 Installing dependencies with uv...")
    if not run_command("uv sync"):
        print("❌ Failed to install dependencies")
        return False
    
    print("✅ Dependencies installed successfully")
    
    # Create .env file if it doesn't exist
    env_file = bridge_dir / ".env"
    if not env_file.exists():
        print("📝 Creating .env file...")
        env_content = """# MT5 Bridge Configuration
MT5_HOST=localhost
MT5_PORT=8080
DEBUG=true
LOG_LEVEL=INFO

# Risk Management
MAX_DAILY_LOSS=100.0
MAX_POSITION_SIZE=0.01
MAX_OPEN_POSITIONS=5

# Webhook Security
WEBHOOK_SECRET=your-webhook-secret-here
"""
        env_file.write_text(env_content)
        print("✅ .env file created")
    else:
        print("✅ .env file already exists")
    
    return True

def test_installation():
    """Test the MT5 bridge installation"""
    print("🧪 Testing installation...")
    
    # Test uv environment
    if not run_command("uv run python --version"):
        print("❌ Failed to run Python in uv environment")
        return False
    
    # Test imports
    test_script = """
import flask
import MetaTrader5 as mt5
import requests
print("✅ All imports successful")
"""
    
    if not run_command(f'uv run python -c "{test_script}"'):
        print("❌ Import test failed")
        return False
    
    print("✅ Installation test passed")
    return True

def print_usage_instructions():
    """Print usage instructions"""
    print("\n" + "="*60)
    print("🎉 MT5 Bridge Setup Complete!")
    print("="*60)
    print()
    print("📋 Next Steps:")
    print()
    print("1. Start the MT5 Bridge:")
    print("   cd mt5-bridge")
    print("   uv run python mt5_bridge.py")
    print()
    print("2. Or use the installed script:")
    print("   uv run mt5-bridge")
    print()
    print("3. Test the bridge:")
    print("   curl http://localhost:8080/health")
    print()
    print("4. Development commands:")
    print("   uv add package-name       # Add new dependency")
    print("   uv remove package-name    # Remove dependency") 
    print("   uv run pytest            # Run tests")
    print("   uv run black .           # Format code")
    print("   uv run ruff check .      # Lint code")
    print()
    print("5. Update dependencies:")
    print("   uv sync --upgrade        # Update all packages")
    print()
    print("📖 Documentation:")
    print("   - MT5 Bridge: mt5-bridge/README.md")
    print("   - UV Usage: https://docs.astral.sh/uv/")
    print()
    print("⚠️  Remember to:")
    print("   - Configure your .env file in mt5-bridge/")
    print("   - Install and configure MT5 terminal")
    print("   - Set up your broker account")

def main():
    """Main setup function"""
    print("🚀 MT5 Trading Bridge Setup with UV")
    print("=" * 40)
    
    # Check Python version
    if not check_python_version():
        sys.exit(1)
    
    # Check if uv is installed, install if not
    if not check_uv_installed():
        if not install_uv():
            print("\n❌ Setup failed: Could not install uv")
            print("💡 Try manual installation:")
            print("   Windows: https://docs.astral.sh/uv/getting-started/installation/#windows")
            print("   macOS/Linux: curl -LsSf https://astral.sh/uv/install.sh | sh")
            sys.exit(1)
    
    # Setup MT5 bridge
    if not setup_mt5_bridge():
        print("\n❌ Setup failed: Could not setup MT5 bridge")
        sys.exit(1)
    
    # Test installation
    if not test_installation():
        print("\n⚠️  Installation completed but tests failed")
        print("   You may need to configure MT5 terminal first")
    
    # Print usage instructions
    print_usage_instructions()

if __name__ == "__main__":
    main() 