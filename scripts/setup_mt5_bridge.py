#!/usr/bin/env python3
"""
Setup script for MT5 HTTP Bridge
Creates virtual environment and installs dependencies.
"""

import os
import sys
import subprocess
import platform
from pathlib import Path

def run_command(cmd, check=True):
    """Run a command and handle errors."""
    print(f"Running: {cmd}")
    try:
        result = subprocess.run(cmd, shell=True, check=check, capture_output=True, text=True)
        if result.stdout:
            print(result.stdout)
        return result
    except subprocess.CalledProcessError as e:
        print(f"Error: {e}")
        if e.stderr:
            print(f"Error output: {e.stderr}")
        if check:
            sys.exit(1)
        return e

def check_python_version():
    """Check if Python version is compatible."""
    if sys.version_info < (3, 8):
        print("Error: Python 3.8 or higher is required")
        sys.exit(1)
    print(f"✅ Python {sys.version_info.major}.{sys.version_info.minor} detected")

def check_windows():
    """Check if running on Windows (required for MT5)."""
    if platform.system() != "Windows":
        print("⚠️ Warning: MetaTrader 5 only runs on Windows")
        print("For development/testing on other platforms, the bridge will handle connection errors gracefully")
    else:
        print("✅ Windows detected - MT5 compatible")

def setup_virtual_environment():
    """Create and setup virtual environment."""
    bridge_dir = Path(__file__).parent.parent / "mt5-bridge"
    venv_dir = bridge_dir / "venv"
    
    print(f"Setting up virtual environment in: {venv_dir}")
    
    # Create virtual environment
    if not venv_dir.exists():
        run_command(f"python3 -m venv {venv_dir}")
        print("✅ Virtual environment created")
    else:
        print("✅ Virtual environment already exists")
    
    # Determine activation script
    if platform.system() == "Windows":
        activate_script = venv_dir / "Scripts" / "activate.bat"
        pip_executable = venv_dir / "Scripts" / "pip.exe"
    else:
        activate_script = venv_dir / "bin" / "activate"
        pip_executable = venv_dir / "bin" / "pip"
    
    # Install requirements
    requirements_file = bridge_dir / "requirements.txt"
    if requirements_file.exists():
        print("Installing Python dependencies...")
        run_command(f"{pip_executable} install --upgrade pip")
        run_command(f"{pip_executable} install -r {requirements_file}")
        print("✅ Dependencies installed")
    else:
        print("❌ requirements.txt not found")
        sys.exit(1)
    
    return venv_dir, activate_script

def create_run_script(venv_dir):
    """Create a run script for the MT5 bridge."""
    bridge_dir = Path(__file__).parent.parent / "mt5-bridge"
    
    if platform.system() == "Windows":
        run_script = bridge_dir / "run_bridge.bat"
        python_executable = venv_dir / "Scripts" / "python.exe"
        
        script_content = f"""@echo off
echo Starting MT5 HTTP Bridge...
cd /d "{bridge_dir}"
"{python_executable}" mt5_bridge.py
pause
"""
    else:
        run_script = bridge_dir / "run_bridge.sh"
        python_executable = venv_dir / "bin" / "python"
        
        script_content = f"""#!/bin/bash
echo "Starting MT5 HTTP Bridge..."
cd "{bridge_dir}"
"{python_executable}" mt5_bridge.py
"""
    
    with open(run_script, 'w') as f:
        f.write(script_content)
    
    if not platform.system() == "Windows":
        os.chmod(run_script, 0o755)
    
    print(f"✅ Run script created: {run_script}")

def main():
    """Main setup function."""
    print("🛠️ Setting up MT5 HTTP Bridge...")
    print("=" * 50)
    
    check_python_version()
    check_windows()
    
    venv_dir, activate_script = setup_virtual_environment()
    create_run_script(venv_dir)
    
    print("\n" + "=" * 50)
    print("✅ MT5 Bridge setup complete!")
    print("\nNext steps:")
    print("1. Ensure MetaTrader 5 is installed and running")
    print("2. Enable 'Allow DLL imports' in MT5 Tools > Options > Expert Advisors")
    print("3. Start the bridge:")
    
    if platform.system() == "Windows":
        print("   - Run: mt5-bridge\\run_bridge.bat")
    else:
        print("   - Run: ./mt5-bridge/run_bridge.sh")
    
    print("4. The bridge will be available at: http://localhost:8080")
    print("5. Test with: curl http://localhost:8080/health")

if __name__ == "__main__":
    main() 