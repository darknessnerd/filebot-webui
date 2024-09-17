const { DataTypes } = require('sequelize');
const sequelize = require('#configs/db');
const File = require('#models/file');
const Parameter = require('#models/parameters');

const Log = sequelize.define('Log', {
    log_id: {
        type: DataTypes.INTEGER,
        primaryKey: true,
        autoIncrement: true,
    },
    file_id: {
        type: DataTypes.INTEGER,
        references: {
            model: File,
            key: 'file_id',
        },
        onDelete: 'CASCADE',
    },
    parameter_id: {
        type: DataTypes.INTEGER,
        references: {
            model: Parameter,
            key: 'parameter_id',
        },
        onDelete: 'CASCADE',
    },
    status: {
        type: DataTypes.STRING,
    },
    log_details: {
        type: DataTypes.TEXT,
    },
    executed_at: {
        type: DataTypes.DATE,
        defaultValue: DataTypes.NOW,
    },
}, {
    tableName: 'logs',
    timestamps: false,
});

module.exports = Log;
