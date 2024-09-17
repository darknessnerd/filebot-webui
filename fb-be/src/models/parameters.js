const { DataTypes } = require('sequelize');
const sequelize = require('#configs/db');
const File = require('#models/file');  // Import the File model to create the association

const Parameter = sequelize.define('Parameter', {
    parameter_id: {
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
        onDelete: 'CASCADE',  // If the related file is deleted, delete the parameters
    },
    db: {
        type: DataTypes.STRING,
    },
    format: {
        type: DataTypes.STRING,
    },
    action: {
        type: DataTypes.STRING,
    },
    filter: {
        type: DataTypes.STRING,
    },
    conflict_resolution: {
        type: DataTypes.STRING,
    },
    created_at: {
        type: DataTypes.DATE,
        defaultValue: DataTypes.NOW,
    },
}, {
    tableName: 'parameters',
    timestamps: false,
});

module.exports = Parameter;
