const { Sequelize, DataTypes } = require('sequelize');
const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

const sequelize = require('#configs/db');
const bodyParser = require('body-parser');
const File = require('#models/file');
const Parameters = require('#models/parameters');
const Logs = require('#models/logs')
const User = require('#models/user')
const Associations = require('#configs/associations')

const cors = require('cors')

// Define input/output directories from environment variables
const inputDirectories = process.env.INPUT_DIRS ? process.env.INPUT_DIRS.split(',') : ['.'];
const outputDirectories = process.env.OUTPUT_DIRS ? process.env.OUTPUT_DIRS.split(',') : ['.', './src'];


// THIS FUNCTION WILL CREATE DUMMY DATA IN DATABASE TABLE
const syncDatabase = async () => {
    try {
        await sequelize.authenticate();
        console.log('Connection has been established successfully.');
        // Sync all models
        await sequelize.sync({ force: true });  // `force: true` drops the tables if they already exist and recreates them
        console.log('Database synced successfully.');
    } catch (error) {
        console.error('Unable to connect to the database:', error);
    }
};

// CREATE APIs URL ENDPOINTS TO CREATE AND DELETE TO DO ITEMS
async function startServer() {
    try {
        await syncDatabase();
        const port = 3001
        const express = require('express')
        const app = express()
        // Use CORS middleware
        app.use(cors());
        app.use(express.json())
        // Middleware
        app.use(bodyParser.json());
        app.use(bodyParser.urlencoded({ extended: true }));
        const publicPath = __dirname + '/dist/';
        app.use(express.static(publicPath));

        app.get('/', function (req,res) {
            res.sendFile(publicPath + "index.html");
        });
        // Endpoint to return predefined input/output directories
        app.get('/directories', (req, res) => {
            res.json({
                inputDirectories,
                outputDirectories
            });
        });
        // List files and directories
        app.get('/explore', (req, res) => {
            const dirPath = req.query.path || '.';

            // Read the directory
            fs.readdir(dirPath, { withFileTypes: true }, (err, files) => {
                if (err) {
                    return res.status(500).json({ error: 'Unable to scan directory' });
                }

                // Map the file details
                const fileDetails = files.map(file => ({
                    name: file.name,
                    path: path.resolve(dirPath), // Ensure correct file path
                    type: file.isDirectory() ? 'directory' : 'file',
                }));

                // Return both the current path and the list of files
                res.json({
                    currentPath: path.resolve(dirPath),
                    name: path.basename(dirPath),
                    path: path.dirname(dirPath),
                    files: fileDetails
                });
            });
        });



        app.post('/execute-filebot', async (req, res) => {
            try {
                const { files, outputDirectory, db, format, action, filter, conflict_resolution, log_level, query, recursive } = req.body;

                // Create a record for parameters
                const parameters = await Parameters.create({
                    db,
                    format,
                    action,
                    filter,
                    conflict_resolution,
                    log_level
                });

                // Initialize an array to store errors
                let errorMessages = [];
                let successMessages = [];

                // Validate and process files and parameters
                for (const file of files) {
                    try {
                        const outputFullPath = path.join(outputDirectory.path, outputDirectory.name);

                        const fileRecord = await File.create({
                            file_name: file.name,
                            file_path: file.path,
                            file_size: file.size, // Assuming file size is provided
                            file_type: file.type // Assuming file type is provided
                        });

                        // Build the FileBot command arguments
                        const args = [
                            '-rename', `${path.join(file.path, file.name)}`,
                            '--db', db,
                            '--action', action,
                            '--conflict', conflict_resolution,
                            '--log', log_level,
                            '--output', outputFullPath,
                            '-non-strict'
                        ];

                        if (format) args.push('--format', format);
                        if (filter) args.push('--filter', filter);
                        if (query) args.push('--q', query);
                        if (recursive) args.push('-r');

                        console.log(args);

                        // Spawn the FileBot process
                        const filebotProcess = spawn('filebot', args);

                        let stdoutData = '';
                        let stderrData = '';

                        // Collect stdout data
                        filebotProcess.stdout.on('data', (data) => {
                            stdoutData += data.toString();
                        });

                        // Collect stderr data
                        filebotProcess.stderr.on('data', (data) => {
                            stderrData += data.toString();
                        });

                        // Wait for the process to finish
                        const exitCode = await new Promise((resolve) => {
                            filebotProcess.on('close', resolve);
                        });

                        // Handle process exit
                        if (exitCode !== 0) {
                            console.error(`FileBot process exited with code ${exitCode}`);
                            console.error(`Error: ${stderrData} Output: ${stdoutData}`);

                            // Collect error message
                            errorMessages.push(`Error processing file "${file.name}": ${stderrData} ${stdoutData}`);
                            continue; // Move to the next file
                        }

                        // Log the success into the database
                        await Logs.create({
                            file_id: fileRecord.file_id,
                            parameter_id: parameters.parameter_id, // Link the right parameter ID
                            status: 'success',
                            log_details: stdoutData,
                        });

                        successMessages.push(`Successfully processed file "${file.name}": ${stdoutData}`);
                    } catch (err) {
                        console.error(`Error processing file "${file.name}":`, err);
                        errorMessages.push(`Error processing file "${file.name}": ${err}`);
                    }
                }

                // After processing all files, send a final response
                if (errorMessages.length > 0) {
                    return res.status(500).json({ success: false, errors: errorMessages, successes: successMessages });
                } else {
                    return res.status(200).json({ success: true, message: 'All files processed successfully.', successes: successMessages });
                }

            } catch (error) {
                console.error('Error:', error);
                return res.status(500).json({ success: false, message: `Server error: ${error.message}` });
            }
        });



        app.listen(port, () => {
            console.log(`App listening on port ${port}`)
        })
    } catch (error) {
        console.error(error);
    }
}
startServer()
